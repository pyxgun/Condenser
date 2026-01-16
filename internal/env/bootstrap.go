package env

import (
	"bufio"
	"condenser/internal/lsm"
	"condenser/internal/store/csm"
	"condenser/internal/store/ilm"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"fmt"
	"os"
	"strings"
)

func NewBootstrapManager() *BootstrapManager {
	return &BootstrapManager{
		filesystemHandler: utils.NewFilesystemExecutor(),
		commandFactory:    utils.NewCommandFactory(),
		ipamStoreHandler:  ipam.NewIpamStore(IpamStorePath),
		ipamHandler:       ipam.NewIpamManager(ipam.NewIpamStore(IpamStorePath)),
		csmStoreHandler:   csm.NewCsmStore(CsmStorePath),
		ilmStoreHandler:   ilm.NewIlmStore(IlmStorePath),
		appArmorHandler:   lsm.NewAppArmorManager(),
	}
}

type BootstrapManager struct {
	filesystemHandler utils.FilesystemHandler
	commandFactory    utils.CommandFactory
	ipamStoreHandler  ipam.IpamStoreHandler
	ipamHandler       ipam.IpamHandler
	csmStoreHandler   csm.CsmStoreHandler
	ilmStoreHandler   ilm.IlmStoreHandler
	appArmorHandler   lsm.AppArmorHandler
}

func (m *BootstrapManager) SetupRuntime() error {
	// 1. create runtime directory
	if err := m.setupRuntimeDirectory(); err != nil {
		return err
	}

	// 2. setup cgroup
	if err := m.setupCgroup(); err != nil {
		return err
	}

	// 4. setup IPAM (IP Address Managemant)
	if err := m.setupIpam(); err != nil {
		return err
	}

	// 3. setup network
	if err := m.setupNetwork(); err != nil {
		return err
	}

	// 5. setup CSM (Container State Management)
	if err := m.setupCsm(); err != nil {
		return err
	}

	// 6. setup ILM (Image Layer Management)
	if err := m.setupIlm(); err != nil {
		return err
	}

	// 7. setup AppArmor
	if err := m.setupAppArmor(); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) setupRuntimeDirectory() error {
	dirs := []string{
		ContainerRootDir,
		ImageRootDir,
		LayerRootDir,
		StoreDir,
	}
	for _, dir := range dirs {
		if err := m.filesystemHandler.MkdirAll(dir, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (m *BootstrapManager) setupCgroup() error {
	// 1. create cgroup runtime directory
	if err := m.setupCgroupDirectory(); err != nil {
		return err
	}

	// 2. enable controllers
	if err := m.enableCgroupControllers(); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) setupCgroupDirectory() error {
	dir := CgroupRuntimeDir
	if err := m.filesystemHandler.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) enableCgroupControllers() error {
	// get current enabled control
	enabled, err := m.readCgroupEnabledControllers()
	if err != nil {
		return nil
	}

	controllers := []string{
		"cpu",
		"memory",
	}
	for _, c := range controllers {
		if enabled[c] {
			continue
		}
		if err := m.writeCgroupController("+" + c); err != nil {
			return err
		}
	}

	return nil
}

func (m *BootstrapManager) readCgroupEnabledControllers() (map[string]bool, error) {
	subtreePath := CgroupSubtreeControlPath
	f, err := m.filesystemHandler.Open(subtreePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	enabled := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		for _, name := range fields {
			enabled[name] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return enabled, nil
}

func (m *BootstrapManager) writeCgroupController(token string) error {
	subtreePath := CgroupSubtreeControlPath
	f, err := m.filesystemHandler.OpenFile(subtreePath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", token); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) setupNetwork() error {
	// 1. create bridge interface
	if err := m.createBridgeInterface(); err != nil {
		return err
	}

	// 2. setup masquerade
	if err := m.createMasqueradeRule(); err != nil {
		return err
	}

	// 3. setup protect rule
	if err := m.createManagementProtectRule(); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) createBridgeInterface() error {
	networkList, err := m.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}

	for _, n := range networkList {
		// check if bridge interface already exist
		check := m.commandFactory.Command("ip", "link", "show", n.Interface)
		if err := check.Run(); err == nil {
			// bridge interface already exist
			return nil
		}

		// create bridge interface
		add := m.commandFactory.Command("ip", "link", "add", n.Interface, "type", "bridge")
		if err := add.Run(); err != nil {
			return fmt.Errorf("ip link add: %w", err)
		}

		// assign address
		assign := m.commandFactory.Command("ip", "addr", "add", n.Address, "dev", n.Interface)
		if err := assign.Run(); err != nil {
			return fmt.Errorf("ip addr add: %w", err)
		}

		// up link
		up := m.commandFactory.Command("ip", "link", "set", n.Interface, "up")
		if err := up.Run(); err != nil {
			return fmt.Errorf("ip link up: %w", err)
		}
	}
	return nil
}

func (m *BootstrapManager) createMasqueradeRule() error {
	hostInterface, err := m.ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}
	runtimeSubnet, err := m.ipamHandler.GetRuntimeSubnet()
	if err != nil {
		return err
	}

	// check if rule already exist
	check := m.commandFactory.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", runtimeSubnet, "-o", hostInterface, "-j", "MASQUERADE")
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	add := m.commandFactory.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", runtimeSubnet, "-o", hostInterface, "-j", "MASQUERADE")
	if err := add.Run(); err != nil {
		return fmt.Errorf("iptables add: %w", err)
	}
	return nil
}

func (m *BootstrapManager) createManagementProtectRule() error {
	runtimeSubnet, err := m.ipamHandler.GetRuntimeSubnet()
	if err != nil {
		return err
	}
	hostAddr, err := m.ipamHandler.GetDefaultInterfaceAddr()
	if err != nil {
		return err
	}
	hostAddr = strings.Split(hostAddr, "/")[0]

	// allow rule for hook traffic: container -> host:7756
	allowHook := m.commandFactory.Command("iptables", "-I", "INPUT", "1", "-s", runtimeSubnet, "-p", "tcp", "-d", hostAddr, "--dport", "7756", "-j", "ACCEPT")
	if err := allowHook.Run(); err != nil {
		return err
	}
	// drop rule for management traffic: container -> host:7755
	dropMgmt := m.commandFactory.Command("iptables", "-I", "INPUT", "2", "-s", runtimeSubnet, "-p", "tcp", "-d", hostAddr, "--dport", "7755", "-j", "DROP")
	if err := dropMgmt.Run(); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) setupIpam() error {
	return m.ipamStoreHandler.SetConfig()
}

func (m *BootstrapManager) setupCsm() error {
	return m.csmStoreHandler.SetContainerState()
}

func (m *BootstrapManager) setupIlm() error {
	return m.ilmStoreHandler.SetConfig()
}

func (m *BootstrapManager) setupAppArmor() error {
	if err := m.appArmorHandler.EnsureRaindDefaultProfile(); err != nil {
		// if apparmor setting failed, runtime ignore apparmor setting
		return nil
	}
	return nil
}
