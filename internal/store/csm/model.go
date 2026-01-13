package csm

import "time"

type ContainerInfo struct {
	ContainerId string    `json:"containerId"`
	State       string    `json:"state"`
	Pid         int       `json:"pid"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ContainerState struct {
	Version    string                   `json:"version"`
	Containers map[string]ContainerInfo `json:"containers"`
}
