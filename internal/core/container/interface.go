package container

type ContainerServiceHandler interface {
	Create(createParameter ServiceCreateModel) (string, error)
	Start(startParameter ServiceStartModel) (string, error)
	Delete(deleteParameter ServiceDeleteModel) (string, error)
	Stop(stopParameter ServiceStopModel) (string, error)
	GetContainerList() ([]ContainerState, error)
	GetContainerById(containerId string) (ContainerState, error)
}

type CgroupServiceHandler interface {
	ChangeCgroupMode(containerId string) error
}
