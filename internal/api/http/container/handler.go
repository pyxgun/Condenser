package container

import (
	"condenser/internal/core/container"
	"net/http"

	"github.com/go-chi/chi/v5"

	apimodel "condenser/internal/api/http/utils"
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
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers [post]
func (h *RequestHandler) CreateContainer(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req CreateContainerRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), CreateContainerResponse{Id: ""})
		return
	}

	// service: create
	result, err := h.serviceHandler.Create(
		container.ServiceCreateModel{
			Image:   req.Image,
			Command: req.Command,
			Port:    req.Port,
			Mount:   req.Mount,
			Network: req.Network,
			Tty:     req.Tty,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), CreateContainerResponse{Id: ""})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "cotainer created", CreateContainerResponse{Id: result})
}

// StartContainer godoc
// @Summary start a container
// @Description start an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Param request body StartContainerRequest true "Start Options"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/start [post]
func (h *RequestHandler) StartContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing container Id", StartContainerResponse{Id: ""})
		return
	}

	// decode request
	var req StartContainerRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), StartContainerResponse{Id: containerId})
		return
	}

	// service: start
	result, err := h.serviceHandler.Start(
		container.ServiceStartModel{
			ContainerId: containerId,
			Tty:         req.Tty,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), StartContainerResponse{Id: containerId})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container started", StartContainerResponse{Id: result})
}

// StopContainer godoc
// @Summary stop a container
// @Description stop an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/stop [post]
func (h *RequestHandler) StopContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing containerId", StopContainerResponse{Id: ""})
		return
	}

	// service: stop
	result, err := h.serviceHandler.Stop(
		container.ServiceStopModel{
			ContainerId: containerId,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), StopContainerResponse{Id: containerId})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container stopped", StopContainerResponse{Id: result})
}

// ExecContainer godoc
// @Summary exec a container
// @Description execute command inside an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Param request body ExecContainerRequest true "Execute Options"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/exec [post]
func (h *RequestHandler) ExecContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing containerId", ExecContainerResponse{Id: ""})
		return
	}

	// decode request
	var req ExecContainerRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), ExecContainerResponse{Id: containerId})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container executed", ExecContainerResponse{Id: containerId})
}

// DeleteContainer godoc
// @Summary delete a container
// @Description delete an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/delete [delete]
func (h *RequestHandler) DeleteContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing containerId", DeleteContainerResponse{Id: ""})
		return
	}

	// service: delete
	result, err := h.serviceHandler.Delete(
		container.ServiceDeleteModel{
			ContainerId: containerId,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), DeleteContainerResponse{Id: containerId})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container deleted", DeleteContainerResponse{Id: result})
}

// GetContainerList godoc
// @Summary get container list
// @Description get all container list
// @Tags containers
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers [get]
func (h *RequestHandler) GetContainerList(w http.ResponseWriter, r *http.Request) {
	// service: get container list
	containerList, err := h.serviceHandler.GetContainerList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve container list failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "retrieve container list success", containerList)
}

// GetContainerById godoc
// @Summary get container info
// @Description get an exitsting container info
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId} [get]
func (h *RequestHandler) GetContainerById(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing container Id", StartContainerResponse{Id: ""})
		return
	}

	// service: get container by id
	containerInfo, err := h.serviceHandler.GetContainerById(containerId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "retrieve container info success", containerInfo)
}
