package image

import (
	"condenser/internal/core/image"
	"net/http"

	apimodel "condenser/internal/api/http/utils"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler: image.NewImageService(),
	}
}

type RequestHandler struct {
	serviceHandler image.ImageServiceHandler
}

// PullImage godoc
// @Summary pull image
// @Description pull image from registry
// @Tags image
// @Accept json
// @Produce json
// @Param request body PullImageRequest true "Target Image"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/images [post]
func (h *RequestHandler) PullImage(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req PullImageRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
	}

	// service
	if err := h.serviceHandler.Pull(
		image.ServicePullModel{
			Image: req.Image,
			Os:    req.Os,
			Arch:  req.Arch,
		},
	); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "pull failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "pull completed", req)
}

// RemoveImage godoc
// @Summary remove image
// @Description remove image from local
// @Tags image
// @Accept json
// @Produce json
// @Param request body RemoveImageRequest true "Target Image"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/images [delete]
func (h *RequestHandler) RemoveImage(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req RemoveImageRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
	}

	// service
	if err := h.serviceHandler.Remove(
		image.ServiceRemoveModel{
			Image: req.Image,
		},
	); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "remove completed", req)
}

// GetImageList godoc
// @Summary get image list
// @Description get image list in local storage
// @Tags image
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/images [get]
func (h *RequestHandler) GetImageList(w http.ResponseWriter, r *http.Request) {
	// service
	imageList, err := h.serviceHandler.GetImageList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "retrieve image list success", imageList)
}
