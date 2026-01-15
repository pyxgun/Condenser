package image

type ImageServiceHandler interface {
	Pull(pullParameter ServicePullModel) error
	Remove(removeParameter ServiceRemoveModel) error
	GetImageConfig(filepath string) (ImageConfigFile, error)
	GetImageList() ([]ImageInfo, error)
}
