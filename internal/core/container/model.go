package container

type ServiceCreateModel struct {
	Image   string
	Command []string
	Port    []string
	Mount   []string
}

type ServiceStartModel struct {
	ContainerId string
	Interactive bool
}

type ServiceDeleteModel struct {
	ContainerId string
}

type ServiceStopModel struct {
	ContainerId string
}
