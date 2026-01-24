package network

type NetworkServiceHandler interface {
	CreateBridgeInterface(ifname string, addr string) error
	CreateMasqueradeRule(src string, dst string) error
	InsertInputRule(num int, ruleModel InputRuleModel, action string) error
	CreateForwardingRule(containerId string, parameter ServiceNetworkModel) error
	RemoveForwardingRule(containerId string, parameter ServiceNetworkModel) error
}
