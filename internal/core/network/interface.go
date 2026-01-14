package network

type NetworkServiceHandler interface {
	CreateForwardingRule(containerId string, parameter ServiceNetworkModel) error
	RemoveForwardingRule(containerId string, parameter ServiceNetworkModel) error
}
