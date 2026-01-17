package runtime

type SpecModel struct {
	Rootfs    string
	Cwd       string
	Command   string
	Namespace []string
	Hostname  string
	Env       []string
	Mount     []string

	HostInterface          string
	BridgeInterface        string
	ContainerInterface     string
	ContainerInterfaceAddr string
	ContainerGateway       string
	ContainerDns           []string

	ImageLayer []string
	UpperDir   string
	WorkDir    string

	CreateRuntimeHook   []string
	CreateContainerHook []string
	StartContainerHook  []string
	PoststartHook       []string
	StopContainerHook   []string
	PoststopHook        []string

	Output string
}

type CreateModel struct {
	ContainerId string
	Tty         bool
}

type StartModel struct {
	ContainerId string
	Tty         bool
}

type DeleteModel struct {
	ContainerId string
}

type StopModel struct {
	ContainerId string
}
