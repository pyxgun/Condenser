package ipam

type IpamStoreHandler interface {
	SetConfig() error
	GetNetworkList() ([]NetworkList, error)
	GetRuntimeSubnet() (string, error)
	GetDefaultInterface() (string, error)
	GetDefaultInterfaceAddr() (string, error)
	GetContainerAddress(containerId string) (string, string, string, error)
	SetForwardInfo(containerId string, sport, dport int, protocol string) error
	GetForwardInfo(containerId string) ([]ForwardInfo, error)
}

type IpamHandler interface {
	Allocate(containerId string, bridge string) (string, error)
	Release(containerId string) error
}
