package container

import (
	"condenser/internal/core/container"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler: container.NewContaierService(),
	}
}

type RequestHandler struct {
	serviceHandler container.ContainerServiceHandler
}

// CreateContainer godoc
// @Summary Create a container
// @Description create a new container
// @Tags containers
// @Accept json
// @Produce json
// @Param request body CreateContainerRequest true "Container Spec"
// @Success 201 {object} ApiResponse
// @Router /v1/containers [post]
func (h *RequestHandler) CreateContainer(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req CreateContainerRequest
	if err := h.decodeRequestBody(r, &req); err != nil {
		h.responsdFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), CreateContainerResponse{Id: ""})
		return
	}

	// service: create
	result, err := h.serviceHandler.Create(
		container.ServiceCreateModel{
			Image:   req.Image,
			Command: req.Command,
		},
	)
	if err != nil {
		h.responsdFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), CreateContainerResponse{Id: ""})
		return
	}

	// encode response
	h.responsdSuccess(w, http.StatusOK, "cotainer created", CreateContainerResponse{Id: result})
}

// StartContainer godoc
// @Summary start a container
// @Description start an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Param request body StartContainerRequest true "Start Options"
// @Success 201 {object} ApiResponse
// @Router /v1/containers/{containerId}/actions/start [post]
func (h *RequestHandler) StartContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		h.responsdFail(w, http.StatusBadRequest, "missing container Id", StartContainerResponse{Id: ""})
		return
	}

	// decode request
	var req StartContainerRequest
	if err := h.decodeRequestBody(r, &req); err != nil {
		h.responsdFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), StartContainerResponse{Id: containerId})
		return
	}

	// service: start
	result, err := h.serviceHandler.Start(
		container.ServiceStartModel{
			ContainerId: containerId,
			Interactive: req.Interactive,
		},
	)
	if err != nil {
		h.responsdFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), StartContainerResponse{Id: containerId})
		return
	}

	// encode response
	h.responsdSuccess(w, http.StatusOK, "container started", StartContainerResponse{Id: result})
}

// StopContainer godoc
// @Summary stop a container
// @Description stop an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 201 {object} ApiResponse
// @Router /v1/containers/{containerId}/actions/stop [post]
func (h *RequestHandler) StopContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		h.responsdFail(w, http.StatusBadRequest, "missing containerId", StopContainerResponse{Id: ""})
		return
	}

	// service: stop
	result, err := h.serviceHandler.Stop(
		container.ServiceStopModel{
			ContainerId: containerId,
		},
	)
	if err != nil {
		h.responsdFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), StopContainerResponse{Id: ""})
		return
	}

	// encode response
	h.responsdSuccess(w, http.StatusOK, "container stopped", StopContainerResponse{Id: result})
}

// ExecContainer godoc
// @Summary exec a container
// @Description execute command inside an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Param request body ExecContainerRequest true "Execute Options"
// @Success 201 {object} ApiResponse
// @Router /v1/containers/{containerId}/actions/exec [post]
func (h *RequestHandler) ExecContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		h.responsdFail(w, http.StatusBadRequest, "missing containerId", ExecContainerResponse{Id: ""})
		return
	}

	// decode request
	var req ExecContainerRequest
	if err := h.decodeRequestBody(r, &req); err != nil {
		h.responsdFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), ExecContainerResponse{Id: containerId})
		return
	}

	// encode response
	h.responsdSuccess(w, http.StatusOK, "container executed", ExecContainerResponse{Id: containerId})
}

// DeleteContainer godoc
// @Summary delete a container
// @Description delete an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} ApiResponse
// @Router /v1/containers/{containerId}/actions/delete [delete]
func (h *RequestHandler) DeleteContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		h.responsdFail(w, http.StatusBadRequest, "missing containerId", DeleteContainerResponse{Id: ""})
		return
	}

	// service: delete
	result, err := h.serviceHandler.Delete(
		container.ServiceDeleteModel{
			ContainerId: containerId,
		},
	)
	if err != nil {
		h.responsdFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), DeleteContainerResponse{Id: containerId})
		return
	}

	// encode response
	h.responsdSuccess(w, http.StatusOK, "container deleted", DeleteContainerResponse{Id: result})
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
