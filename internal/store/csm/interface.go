package csm

type CsmStoreHandler interface {
	SetContainerState() error
}

type CsmHandler interface {
	StoreContainer(containerId string, state string, pid int, repo, ref string, command []string) error
	RemoveContainer(containerId string) error
	UpdateContainer(containerId string, state string, pid int) error
	GetContainerList() ([]ContainerInfo, error)
	GetContainerById(containerId string) (ContainerInfo, error)
}
