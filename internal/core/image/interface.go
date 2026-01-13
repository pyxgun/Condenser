package image

type ImageServiceHandler interface {
	PullImage(pullParameter ServicePullModel) error
}
