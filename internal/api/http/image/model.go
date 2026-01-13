package image

type PullImageRequest struct {
	Image string `json:"image" example:"alpine:latest"`
	Os    string `json:"os" example:"linux"`
	Arch  string `json:"arch" example:"arm64"`
}

type ApiResponse struct {
	Status  string `json:"status"` // success | fail
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
