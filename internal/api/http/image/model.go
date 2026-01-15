package image

type PullImageRequest struct {
	Image string `json:"image" example:"alpine:latest"`
	Os    string `json:"os" example:"linux"`
	Arch  string `json:"arch" example:"arm64"`
}

type RemoveImageRequest struct {
	Image string `json:"image" example:"alpine:latest"`
}
