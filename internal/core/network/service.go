package network

import (
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"slices"
	"strconv"
	"strings"
)

func NewNetworkService() *NetworkService {
	return &NetworkService{
		commandFactory: utils.NewCommandFactory(),
		ipamHandler:    ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
	}
}

type NetworkService struct {
	commandFactory utils.CommandFactory
	ipamHandler    ipam.IpamHandler
}

func (s *NetworkService) CreateBridgeInterface(ifname string, addr string) error {
	// check if bridge already created
	check := s.commandFactory.Command("ip", "link", "show", ifname)
	if err := check.Run(); err == nil {
		// bridge already created, return
		return nil
	}

	// create bridge
	addLink := s.commandFactory.Command("ip", "link", "add", ifname, "type", "bridge")
	if err := addLink.Run(); err != nil {
		return err
	}
	// assign address
	assignAddr := s.commandFactory.Command("ip", "addr", "add", addr, "dev", ifname)
	if err := assignAddr.Run(); err != nil {
		return err
	}
	// up link
	upLink := s.commandFactory.Command("ip", "link", "set", ifname, "up")
	if err := upLink.Run(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) CreateMasqueradeRule(src string, dst string) error {
	// check if rule already exist
	check := s.commandFactory.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", src, "-o", dst, "-j", "MASQUERADE")
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	add := s.commandFactory.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", src, "-o", dst, "-j", "MASQUERADE")
	if err := add.Run(); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) InsertInputRule(num int, ruleModel InputRuleModel, action string) error {
	ruleParam := []string{"-s", ruleModel.SourceAddr, "-d", ruleModel.DestAddr, "-j", action}
	if ruleModel.Protocol != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-p", ruleModel.Protocol})
	}
	if ruleModel.SourcePort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--sport", strconv.Itoa(ruleModel.SourcePort)})
	}
	if ruleModel.DestPort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--dport", strconv.Itoa(ruleModel.DestPort)})
	}

	// check if rule already exist
	checkCmd := slices.Concat([]string{"iptables", "-C", "INPUT"}, ruleParam)
	check := s.commandFactory.Command(checkCmd[0], checkCmd[1:]...)
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	addRuleCmd := slices.Concat([]string{"iptables", "-I", "INPUT", strconv.Itoa(num)}, ruleParam)
	addRule := s.commandFactory.Command(addRuleCmd[0], addRuleCmd[1:]...)
	if err := addRule.Run(); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) CreateForwardingRule(containerId string, parameter ServiceNetworkModel) error {
	// get container address
	host, bridge, addr, err := s.getContainerAddress(containerId)
	if err != nil {
		return err
	}

	// set rules
	if err := s.setForwardRules(host, bridge, addr, parameter); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) RemoveForwardingRule(containerId string, parameter ServiceNetworkModel) error {
	// get container address
	host, bridge, addr, err := s.getContainerAddress(containerId)
	if err != nil {
		return err
	}

	// remove rules
	if err := s.deleteForwardRules(host, bridge, addr, parameter); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) getContainerAddress(containerId string) (string, string, string, error) {
	host, bridge, addr, err := s.ipamHandler.GetContainerAddress(containerId)
	if err != nil {
		return "", "", "", err
	}
	return host, bridge, addr, nil
}

func (s *NetworkService) setForwardRules(hostInterface string, bridgeInterface string, containerAddr string, portParam ServiceNetworkModel) error {
	// 1. dnat
	dnatRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-A", "PREROUTING",
		"-i", hostInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatRule := s.commandFactory.Command(dnatRuleCmd[0], dnatRuleCmd[1:]...)
	if err := dnatRule.Run(); err != nil {
		return err
	}

	// 2. allow forward: in
	forwardInCmd := []string{
		"iptables",
		"-A", "FORWARD",
		"-i", hostInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	forwardIn := s.commandFactory.Command(forwardInCmd[0], forwardInCmd[1:]...)
	if err := forwardIn.Run(); err != nil {
		return err
	}

	// 3. allow forward: out
	forwardOutCmd := []string{
		"iptables",
		"-A", "FORWARD",
		"-o", hostInterface,
		"-i", bridgeInterface,
		"-p", portParam.Protocol,
		"--sport", portParam.HostPort,
		"-s", containerAddr,
		"-j", "ACCEPT",
	}
	forwardOut := s.commandFactory.Command(forwardOutCmd[0], forwardOutCmd[1:]...)
	if err := forwardOut.Run(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) deleteForwardRules(hostInterface string, bridgeInterface string, containerAddr string, portParam ServiceNetworkModel) error {
	// 1. dnat
	dnatRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-D", "PREROUTING",
		"-i", hostInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatRule := s.commandFactory.Command(dnatRuleCmd[0], dnatRuleCmd[1:]...)
	if err := dnatRule.Run(); err != nil {
		return err
	}

	// 2. allow forward: in
	forwardInCmd := []string{
		"iptables",
		"-D", "FORWARD",
		"-i", hostInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	forwardIn := s.commandFactory.Command(forwardInCmd[0], forwardInCmd[1:]...)
	if err := forwardIn.Run(); err != nil {
		return err
	}

	// 3. allow forward: out
	forwardOutCmd := []string{
		"iptables",
		"-D", "FORWARD",
		"-o", hostInterface,
		"-i", bridgeInterface,
		"-p", portParam.Protocol,
		"--sport", portParam.HostPort,
		"-s", containerAddr,
		"-j", "ACCEPT",
	}
	forwardOut := s.commandFactory.Command(forwardOutCmd[0], forwardOutCmd[1:]...)
	if err := forwardOut.Run(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) CreateChain(chainName string) error {
	// check if chain already exist
	check := s.commandFactory.Command("iptables", "-L", chainName)
	if err := check.Run(); err == nil {
		// chain already exist
		return nil
	}

	// create chain
	create := s.commandFactory.Command("iptables", "-N", chainName)
	if err := create.Run(); err != nil {
		return err
	}

	// clear chain
	clear := s.commandFactory.Command("iptables", "-F", chainName)
	if err := clear.Run(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) InsertForwardRule(chainName string) error {
	// check if forward rule already exist
	check := s.commandFactory.Command("iptables", "-C", "FORWARD", "-j", chainName)
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// insert rule
	insertRule := s.commandFactory.Command("iptables", "-I", "FORWARD", "1", "-j", chainName)
	if err := insertRule.Run(); err != nil {
		return err
	}

	return nil
}

type RuleModel struct {
	Conntrack       bool
	Ctstate         []string
	Physdev         bool
	PhysdevIsBridge bool
	InputDev        string
	OutputDev       string
	InputPhysdev    string
	OutputPhysdev   string
	Protocol        string
	SourcePort      int
	DestPort        int

	NflogGroup  int
	NflogPrefix string
}

func (s *NetworkService) AddRuleToChain(chainName string, ruleModel RuleModel, action string) error {
	ruleParam := []string{chainName, "-j", action}
	if ruleModel.Conntrack {
		ruleParam = slices.Concat(ruleParam, []string{"-m", "conntrack"})
	}
	if len(ruleModel.Ctstate) > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--ctstate", strings.Join(ruleModel.Ctstate, ",")})
	}
	if ruleModel.Physdev {
		ruleParam = slices.Concat(ruleParam, []string{"-m", "physdev"})
	}
	if ruleModel.PhysdevIsBridge {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-is-bridged"})
	}
	if ruleModel.InputDev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-i", ruleModel.InputDev})
	}
	if ruleModel.OutputDev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-o", ruleModel.OutputDev})
	}
	if ruleModel.InputPhysdev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-in", ruleModel.InputPhysdev})
	}
	if ruleModel.OutputPhysdev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-out", ruleModel.OutputPhysdev})
	}
	if ruleModel.Protocol != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-p", ruleModel.Protocol})
	}
	if ruleModel.SourcePort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--sport", strconv.Itoa(ruleModel.SourcePort)})
	}
	if ruleModel.DestPort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--dport", strconv.Itoa(ruleModel.DestPort)})
	}
	if ruleModel.NflogGroup > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--nflog-group", strconv.Itoa(ruleModel.NflogGroup)})
	}
	if ruleModel.NflogPrefix != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--nflog-prefix", ruleModel.NflogPrefix})
	}

	// check if rule already exist
	checkCmd := slices.Concat([]string{"iptables", "-C"}, ruleParam)
	check := s.commandFactory.Command(checkCmd[0], checkCmd[1:]...)
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	addRuleCmd := slices.Concat([]string{"iptables", "-A"}, ruleParam)
	addRule := s.commandFactory.Command(addRuleCmd[0], addRuleCmd[1:]...)
	if err := addRule.Run(); err != nil {
		return err
	}
	return nil
}
