package csm

type CsmStoreHandler interface {
	SetContainerState() error
}

type CsmHandler interface {
	StoreContainer(containerId string, state string, pid int) error
	RemoveContainer(containerId string) error
	UpdateContainer(containerId string, state string, pid int) error
}
