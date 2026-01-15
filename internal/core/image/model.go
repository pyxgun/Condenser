package image

import "time"

type ServicePullModel struct {
	Image string
	Os    string
	Arch  string
}

type ServiceRemoveModel struct {
	Image string
}

// image bundle object
type ImageConfigObject struct {
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	Entrypoint []string `json:"Entrypoint"`
	WorkingDir string   `json:"WorkingDir"`
}

type ImageConfigFile struct {
	Config ImageConfigObject `json:"config"`
}

type ImageInfo struct {
	Repository string    `json:"repository"`
	Reference  string    `json:"reference"`
	CreatedAt  time.Time `json:"createdAt"`
}
