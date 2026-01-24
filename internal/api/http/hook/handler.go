package http

import (
	"condenser/internal/core/hook"
	"encoding/json"
	"io"
	"net/http"

	apimodel "condenser/internal/api/http/utils"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		hookServiceHandler: hook.NewHookService(),
	}
}

type RequestHandler struct {
	hookServiceHandler hook.HookServiceHandler
}

// ApplyHook godoc
// @Summary apply hook
// @Description apply hook from droplet
// @Tags hooks
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/hooks/droplet [post]
func (h *RequestHandler) ApplyHook(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get("X-Hook-Event")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "read hook body failed: "+err.Error(), nil)
		return
	}
	var st hook.ServiceStateModel
	if err := json.Unmarshal(body, &st); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
		return
	}

	// service: hook
	if err := h.hookServiceHandler.HookAction(st, eventType); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service hook failed: "+err.Error(), nil)
	}

	apimodel.RespondSuccess(w, http.StatusOK, "hook applied", nil)
}
