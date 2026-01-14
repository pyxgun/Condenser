package network

import (
	"condenser/internal/env"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
)

func NewNetworkService() *NetworkService {
	return &NetworkService{
		commandFactory:   utils.NewCommandFactory(),
		ipamStoreHandler: ipam.NewIpamStore(env.IpamStorePath),
	}
}

type NetworkService struct {
	commandFactory   utils.CommandFactory
	ipamStoreHandler ipam.IpamStoreHandler
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
	host, bridge, addr, err := s.ipamStoreHandler.GetContainerAddress(containerId)
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
