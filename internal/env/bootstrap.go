package env

import (
	"bufio"
	"condenser/internal/core/network"
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
		networkHandler:    network.NewNetworkService(),
		ipamStoreHandler:  ipam.NewIpamStore(utils.IpamStorePath),
		ipamHandler:       ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		csmStoreHandler:   csm.NewCsmStore(utils.CsmStorePath),
		ilmStoreHandler:   ilm.NewIlmStore(utils.IlmStorePath),
		appArmorHandler:   lsm.NewAppArmorManager(),
	}
}

type BootstrapManager struct {
	filesystemHandler utils.FilesystemHandler
	commandFactory    utils.CommandFactory
	networkHandler    network.NetworkServiceHandler
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

	// 3. setup IPAM (IP Address Managemant)
	if err := m.setupIpam(); err != nil {
		return err
	}

	// 4. setup CSM (Container State Management)
	if err := m.setupCsm(); err != nil {
		return err
	}

	// 5. setup ILM (Image Layer Management)
	if err := m.setupIlm(); err != nil {
		return err
	}

	// 6. setup network
	if err := m.setupNetwork(); err != nil {
		return err
	}

	// 7. setup network chain
	if err := m.setupNetworkChain(); err != nil {
		return err
	}

	// 8. setup AppArmor
	if err := m.setupAppArmor(); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) setupRuntimeDirectory() error {
	dirs := []string{
		utils.ContainerRootDir,
		utils.ImageRootDir,
		utils.LayerRootDir,
		utils.StoreDir,
		utils.AuditLogDir,
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
	dir := utils.CgroupRuntimeDir
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
	subtreePath := utils.CgroupSubtreeControlPath
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
	subtreePath := utils.CgroupSubtreeControlPath
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

	// 4. setup chain

	return nil
}

func (m *BootstrapManager) createBridgeInterface() error {
	networkList, err := m.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}

	for _, n := range networkList {
		if err := m.networkHandler.CreateBridgeInterface(n.Interface, n.Address); err != nil {
			return err
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

	if err := m.networkHandler.CreateMasqueradeRule(runtimeSubnet, hostInterface); err != nil {
		return err
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
	if err := m.networkHandler.InsertInputRule(
		1,
		network.InputRuleModel{
			SourceAddr: runtimeSubnet,
			DestAddr:   hostAddr,
			Protocol:   "tcp",
			DestPort:   7756,
		},
		"ACCEPT",
	); err != nil {
		return err
	}

	// drop rule for management traffic: container -> host:7755
	if err := m.networkHandler.InsertInputRule(
		2,
		network.InputRuleModel{
			SourceAddr: runtimeSubnet,
			DestAddr:   hostAddr,
			Protocol:   "tcp",
			DestPort:   7755,
		},
		"DROP",
	); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) setupNetworkChain() error {
	// raind default chains
	// 1. RAIND-ROOT
	//      manage all Raind Rules
	// 2. RAIND-EW
	//      manage East-West(contaier-to-container) traffic rules
	// 3. RAIND-NS-OBS
	//      manage North-West(container-to-external) traffic rules
	//      this chain is all accept and log traffic default
	// 4. RAIND-NS-ENF
	//      manage North-West(container-to-external) traffic rules
	//      this chain is all denay and allow explicit

	// == setup predefined rules ==//
	networkList, err := m.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}

	// TODO: implement Observe/Enforce mode
	enforce := false
	// TODO: implement Nflog mode
	nflog := true

	// 1. create chain
	if err := m.createChain(); err != nil {
		return err
	}

	// 2. insert chain to forward
	if err := m.insertForward(); err != nil {
		return err
	}

	// 3. build RAIND-ROOT chain
	if err := m.buildRaindRootChain(enforce); err != nil {
		return err
	}

	// 4. build RAIND-EW chain
	if err := m.buildRaindEWChain(networkList, nflog); err != nil {
		return err
	}

	// 5. build RAIND-NS-OBS chain
	if err := m.buildRaindNSObserveChain(networkList, nflog); err != nil {
		return err
	}

	// 6. build RAIND-NS-ENF chain
	if err := m.buildRaindNSEnforceChain(networkList, nflog); err != nil {
		return err
	}

	// == setup user-defined rules ==

	return nil
}

func (m *BootstrapManager) createChain() error {
	chains := []string{
		"RAIND-ROOT",
		"RAIND-EW",
		"RAIND-NS-OBS",
		"RAIND-NS-ENF",
	}
	for _, c := range chains {
		if err := m.networkHandler.CreateChain(c); err != nil {
			return err
		}
	}
	return nil
}

func (m *BootstrapManager) insertForward() error {
	if err := m.networkHandler.InsertForwardRule("RAIND-ROOT"); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) buildRaindRootChain(enforce bool) error {
	chainName := "RAIND-ROOT"
	// 1. allow return traffic (ESTABLISHED,RELATED)
	if err := m.networkHandler.AddRuleToChain(
		chainName,
		network.RuleModel{
			Conntrack: true,
			Ctstate:   []string{"ESTABLISHED", "RELATED"},
		},
		"ACCEPT",
	); err != nil {
		return err
	}

	// 2. forward to RAIND-EW
	if err := m.networkHandler.AddRuleToChain(
		chainName,
		network.RuleModel{},
		"RAIND-EW",
	); err != nil {
		return err
	}

	if enforce {
		// TODO: implement Observe/Enforce mode
	} else {
		// 3. forward  to RAIND-NS-OBS
		if err := m.networkHandler.AddRuleToChain(
			chainName,
			network.RuleModel{},
			"RAIND-NS-OBS",
		); err != nil {
			return err
		}
	}

	// 4. other: return
	if err := m.networkHandler.AddRuleToChain(
		chainName,
		network.RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) buildRaindEWChain(networkList []ipam.NetworkList, nflog bool) error {
	chainName := "RAIND-EW"

	// 1. if NFLOG mode is enabled, add NFLOG entry
	if nflog {
		for _, n := range networkList {
			if err := m.networkHandler.AddRuleToChain(
				chainName,
				network.RuleModel{
					Conntrack:       true,
					Ctstate:         []string{"NEW"},
					Physdev:         true,
					PhysdevIsBridge: true,
					InputDev:        n.Interface,
					OutputDev:       n.Interface,
					NflogGroup:      10,
					NflogPrefix:     "RAIND-EW-DENY",
				},
				"NFLOG",
			); err != nil {
				return err
			}
		}
	}

	// 2. deny all inter-container traffic
	for _, n := range networkList {
		if err := m.networkHandler.AddRuleToChain(
			chainName,
			network.RuleModel{
				InputDev:  n.Interface,
				OutputDev: n.Interface,
			},
			"DROP",
		); err != nil {
			return err
		}
	}

	// 3. other: return
	if err := m.networkHandler.AddRuleToChain(
		chainName,
		network.RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) buildRaindNSObserveChain(networkList []ipam.NetworkList, nflog bool) error {
	chainName := "RAIND-NS-OBS"
	hostInterface, err := m.ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}

	// 1. if NFLOG mode is enabled, add NFLOG entry
	if nflog {
		for _, n := range networkList {
			if err := m.networkHandler.AddRuleToChain(
				chainName,
				network.RuleModel{
					Conntrack:   true,
					Ctstate:     []string{"NEW"},
					InputDev:    n.Interface,
					OutputDev:   hostInterface,
					NflogGroup:  11,
					NflogPrefix: "RAIND-NS-OBS",
				},
				"NFLOG",
			); err != nil {
				return err
			}
		}
	}

	// 2. allow container to external traffic
	for _, n := range networkList {
		if err := m.networkHandler.AddRuleToChain(
			chainName,
			network.RuleModel{
				InputDev:  n.Interface,
				OutputDev: hostInterface,
			},
			"ACCEPT",
		); err != nil {
			return err
		}
	}

	// 3. other: return
	if err := m.networkHandler.AddRuleToChain(
		chainName,
		network.RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) buildRaindNSEnforceChain(networkList []ipam.NetworkList, nflog bool) error {
	chainName := "RAIND-NS-ENF"
	hostInterface, err := m.ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}

	// 1. if NFLOG mode is enabled, add NFLOG entry
	if nflog {
		for _, n := range networkList {
			if err := m.networkHandler.AddRuleToChain(
				chainName,
				network.RuleModel{
					Conntrack:   true,
					Ctstate:     []string{"NEW"},
					InputDev:    n.Interface,
					OutputDev:   hostInterface,
					NflogGroup:  12,
					NflogPrefix: "RAIND-NS-ENF",
				},
				"NFLOG",
			); err != nil {
				return err
			}
		}
	}

	// 2. allow container to external traffic
	for _, n := range networkList {
		if err := m.networkHandler.AddRuleToChain(
			chainName,
			network.RuleModel{
				InputDev:  n.Interface,
				OutputDev: hostInterface,
			},
			"DROP",
		); err != nil {
			return err
		}
	}

	// 3. other: return
	if err := m.networkHandler.AddRuleToChain(
		chainName,
		network.RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}
