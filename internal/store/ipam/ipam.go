package ipam

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func NewIpamManager(ipamStore *IpamStore) *IpamManager {
	return &IpamManager{
		ipamStore: ipamStore,
	}
}

type IpamManager struct {
	ipamStore *IpamStore
}

func (m *IpamManager) Allocate(containerId string, bridge string) (string, error) {
	var allocated string

	err := m.ipamStore.withLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			if p.Interface == bridge {
				if p.Subnet == "" || p.Address == "" {
					return fmt.Errorf("ipam not configured")
				}
				_, ipnet, _ := net.ParseCIDR(p.Subnet)
				gw := net.ParseIP(strings.Split(p.Address, "/")[0]).To4()
				if gw == nil {
					return fmt.Errorf("gateway must be ipv4")
				}
				next, err := findFreeIpv4(ipnet, gw, p.Allocations)
				if err != nil {
					return err
				}
				ipStr := next.String()
				p.Allocations[ipStr] = Allocation{
					ContainerId: containerId,
					AssignedAt:  time.Now(),
				}
				allocated = ipStr
				return nil
			}
		}
		return fmt.Errorf("target bridge not configured: %s", bridge)
	})
	return allocated, err
}

func (m *IpamManager) Release(containerId string) error {
	return m.ipamStore.withLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			for ip, a := range p.Allocations {
				if a.ContainerId == containerId {
					delete(p.Allocations, ip)
					return nil
				}
			}
		}
		return fmt.Errorf("allocation not found for containerId=%s", containerId)
	})
}

func findFreeIpv4(ipnet *net.IPNet, gateway net.IP, alloc map[string]Allocation) (net.IP, error) {
	network := ipnet.IP.To4()
	if network == nil {
		return nil, fmt.Errorf("ipv4 only supported")
	}
	start := incIP(network)       // network +1
	bcast := broadcastIPv4(ipnet) // reserve: broadcast address

	// search stat
	cursor := start
	for i := 0; i < 1<<24; i++ {
		if !ipnet.Contains(cursor) {
			cursor = start
		}
		// reserve: network, gateway, broadcast
		if cursor.Equal(network) || cursor.Equal(gateway) || cursor.Equal(bcast) {
			cursor = incIP(cursor)
			continue
		}
		if _, used := alloc[cursor.String()]; !used {
			return cursor, nil
		}
		cursor = incIP(cursor)
	}

	return nil, fmt.Errorf("no free ip in subnet %s", ipnet.String())
}

func incIP(ip net.IP) net.IP {
	v := make(net.IP, len(ip))
	copy(v, ip)
	v[3]++
	for i := 3; i >= 0; i-- {
		if v[i] != 0 {
			break
		}
		if i > 0 {
			v[i-1]++
		}
	}
	return v
}

func broadcastIPv4(ipnet *net.IPNet) net.IP {
	ip := ipnet.IP.To4()
	mask := ipnet.Mask
	b := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		b[i] = ip[i] | ^mask[i]
	}
	return b
}
