package csm

import "time"

type ContainerInfo struct {
	ContainerId string    `json:"containerId"`
	State       string    `json:"state"`
	Pid         int       `json:"pid"`
	Repository  string    `json:"imageRepository"`
	Reference   string    `json:"imageReference"`
	Command     []string  `json:"command"`
	CreatingAt  time.Time `json:"creatingAt"`
	CreatedAt   time.Time `json:"createdAt"`
	StartedAt   time.Time `json:"statedAt"`
	StoppedAt   time.Time `json:"stoppedAt"`
}

type ContainerState struct {
	Version    string                   `json:"version"`
	Containers map[string]ContainerInfo `json:"containers"`
}
