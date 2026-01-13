package container

// == create ==
type CreateContainerRequest struct {
	Image   string   `json:"image" example:"alpine:latest"`
	Command []string `json:"command,omitempty" example:"'/bin/sh','-c','echo hello; sleep 60'"`
}

type CreateContainerResponse struct {
	Id string `json:"id"`
}

// == start ==
type StartContainerRequest struct {
	Interactive bool `json:"interactive" example:"true"`
}

type StartContainerResponse struct {
	Id string `json:"id"`
}

// == stop ==
type StopContainerResponse struct {
	Id string `json:"id"`
}

// == exec ==
type ExecContainerRequest struct {
	Command     []string `json:"command" example:"'/bin/sh','-c','echo hello'"`
	Interactive bool     `json:"interactive" example:"true"`
}

type ExecContainerResponse struct {
	Id string `json:"id"`
}

// == delete ==
type DeleteContainerResponse struct {
	Id string `json:"id"`
}

type ApiResponse struct {
	Status  string `json:"status"` // success | fail
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
