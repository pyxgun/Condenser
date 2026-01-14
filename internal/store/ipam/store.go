package ipam

import (
	"condenser/internal/utils"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

func NewIpamStore(path string) *IpamStore {
	return &IpamStore{
		path:              path,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type IpamStore struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *IpamStore) withLock(fn func(st *IpamState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()

	if err := s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_UN)

	st, err := s.loadOrInit()
	if err != nil {
		return err
	}

	if err := fn(st); err != nil {
		return err
	}

	return s.atomicSave(st)
}

func (s *IpamStore) loadOrInit() (*IpamState, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)

	if err != nil {
		defaultHostInterface, getIfErr := GetDefaultInterfaceIpv4()
		if getIfErr != nil {
			return nil, getIfErr
		}
		defaultHostInterfaceAddr, getIfAddrErr := GetDefaultInterfaceAddressIpv4(defaultHostInterface)
		if getIfAddrErr != nil {
			return nil, getIfAddrErr
		}
		if s.filesystemHandler.IsNotExist(err) {
			// ipam state file not exist
			return &IpamState{
				Version:           "0.1.0",
				RuntimeSubnet:     "10.166.0.0/16",
				HostInterface:     defaultHostInterface,
				HostInterfaceAddr: defaultHostInterfaceAddr,
				Pools: []Pool{
					{
						Interface:   "raind0",
						Subnet:      "10.166.0.0/24",
						Address:     "10.166.0.254/24",
						Allocations: map[string]Allocation{},
					},
				},
			}, nil
		}
		return nil, err
	}

	var st IpamState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("ipam state json broken: %w", err)
	}
	for _, p := range st.Pools {
		if p.Allocations == nil {
			p.Allocations = map[string]Allocation{}
		}
	}
	return &st, nil
}

func (s *IpamStore) atomicSave(st *IpamState) error {
	tmp := s.path + ".tmp"

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	f, err := s.filesystemHandler.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return s.filesystemHandler.Rename(tmp, s.path)
}

func (s *IpamStore) SetConfig() error {
	defaultHostInterface, err := GetDefaultInterfaceIpv4()
	if err != nil {
		return err
	}

	defaultHostInterfaceAddr, err := GetDefaultInterfaceAddressIpv4(defaultHostInterface)
	if err != nil {
		return err
	}

	return s.withLock(func(st *IpamState) error {
		st.Version = "0.1.0"
		st.RuntimeSubnet = "10.166.0.0/16"
		st.HostInterface = defaultHostInterface
		st.HostInterfaceAddr = defaultHostInterfaceAddr
		if len(st.Pools) == 0 {
			st.Pools = append(st.Pools, Pool{
				Interface:   "raind0",
				Subnet:      "10.166.0.0/24",
				Address:     "10.166.0.254/24",
				Allocations: map[string]Allocation{},
			})
		}
		return nil
	})
}

func (s *IpamStore) GetNetworkList() ([]NetworkList, error) {
	var networkList []NetworkList

	err := s.withLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			networkList = append(networkList, NetworkList{
				Interface: p.Interface,
				Address:   p.Address,
			})
		}
		if len(networkList) == 0 {
			return fmt.Errorf("network is not configured")
		}
		return nil
	})
	return networkList, err
}

func (s *IpamStore) GetRuntimeSubnet() (string, error) {
	var runtimeSubnet string

	err := s.withLock(func(st *IpamState) error {
		runtimeSubnet = st.RuntimeSubnet
		if runtimeSubnet == "" {
			return fmt.Errorf("runtime subnet is not configured")
		}
		return nil
	})
	return runtimeSubnet, err
}

func (s *IpamStore) GetDefaultInterface() (string, error) {
	var defaultInterface string

	err := s.withLock(func(st *IpamState) error {
		defaultInterface = st.HostInterface
		if defaultInterface == "" {
			return fmt.Errorf("default interface is not configured")
		}
		return nil
	})
	return defaultInterface, err
}

func (s *IpamStore) GetDefaultInterfaceAddr() (string, error) {
	var defaultInterfaceAddr string

	err := s.withLock(func(st *IpamState) error {
		defaultInterfaceAddr = st.HostInterfaceAddr
		if defaultInterfaceAddr == "" {
			return fmt.Errorf("default interface address is not configured")
		}
		return nil
	})
	return defaultInterfaceAddr, err
}

func (s *IpamStore) GetContainerAddress(containerId string) (string, string, string, error) {
	var (
		containerAddr   string
		hostInterface   string
		bridgeInterface string
	)

	err := s.withLock(func(st *IpamState) error {
		hostInterface = st.HostInterface
		for _, p := range st.Pools {
			for addr, info := range p.Allocations {
				if info.ContainerId == containerId {
					bridgeInterface = p.Interface
					containerAddr = addr
					return nil
				}
			}
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return hostInterface, bridgeInterface, containerAddr, err
}

func (s *IpamStore) SetForwardInfo(containerId string, sport, dport int, protocol string) error {
	err := s.withLock(func(st *IpamState) error {
		for i := range st.Pools {
			p := st.Pools[i]
			if p.Allocations == nil {
				continue
			}

			for addr, info := range p.Allocations {
				if info.ContainerId != containerId {
					continue
				}

				fi := ForwardInfo{
					HostPort:      sport,
					ContainerPort: dport,
					Protocol:      protocol,
				}

				alloc := p.Allocations[addr]
				alloc.Forwards = append(alloc.Forwards, fi)
				p.Allocations[addr] = alloc

				return nil
			}
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return err
}

func (s *IpamStore) GetForwardInfo(containerId string) ([]ForwardInfo, error) {
	var forwards []ForwardInfo
	err := s.withLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			if p.Allocations == nil {
				continue
			}
			for _, info := range p.Allocations {
				if info.ContainerId != containerId {
					continue
				}
				for _, f := range info.Forwards {
					forwards = append(forwards, f)
				}
			}
		}
		return nil
	})
	return forwards, err
}
