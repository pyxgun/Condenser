package ipam

type IpamStoreHandler interface {
	SetConfig() error
	GetNetworkList() ([]NetworkList, error)
	GetRuntimeSubnet() (string, error)
	GetDefaultInterface() (string, error)
	GetDefaultInterfaceAddr() (string, error)
}

type IpamHandler interface {
	Allocate(containerId string, bridge string) (string, error)
	Release(containerId string) error
}
