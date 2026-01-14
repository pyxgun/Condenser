package image

import (
	"condenser/internal/core/image"
	"encoding/json"
	"net/http"
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
// @Success 201 {object} ApiResponse
// @Router /v1/images [post]
func (h *RequestHandler) PullImage(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req PullImageRequest
	if err := h.decodeRequestBody(r, &req); err != nil {
		h.responsdFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
	}

	// service
	if err := h.serviceHandler.Pull(
		image.ServicePullModel{
			Image: req.Image,
			Os:    req.Os,
			Arch:  req.Arch,
		},
	); err != nil {
		h.responsdFail(w, http.StatusInternalServerError, "pull failed: "+err.Error(), nil)
		return
	}

	// encode response
	h.responsdSuccess(w, http.StatusOK, "pull completed", req)
}

// RemoveImage godoc
// @Summary remove image
// @Description remove image from local
// @Tags image
// @Accept json
// @Produce json
// @Param request body RemoveImageRequest true "Target Image"
// @Success 201 {object} ApiResponse
// @Router /v1/images [delete]
func (h *RequestHandler) RemoveImage(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req RemoveImageRequest
	if err := h.decodeRequestBody(r, &req); err != nil {
		h.responsdFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
	}

	// service
	if err := h.serviceHandler.Remove(
		image.ServiceRemoveModel{
			Image: req.Image,
		},
	); err != nil {
		h.responsdFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}

	// encode response
	h.responsdSuccess(w, http.StatusOK, "remove completed", req)
}

func (h *RequestHandler) decodeRequestBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

func (h *RequestHandler) writeJson(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *RequestHandler) responsdSuccess(w http.ResponseWriter, statusCode int, message string, data any) {
	h.writeJson(w, statusCode, ApiResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func (h *RequestHandler) responsdFail(w http.ResponseWriter, statusCode int, message string, data any) {
	h.writeJson(w, statusCode, ApiResponse{
		Status:  "fail",
		Message: message,
		Data:    data,
	})
}
