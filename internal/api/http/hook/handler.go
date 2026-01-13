package http

import (
	"condenser/internal/core/hook"
	"encoding/json"
	"io"
	"net/http"
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
// @Tags Hooks
// @Success 200 {object} ApiResponse
// @Router /v1/hooks/droplet [post]
func (h *RequestHandler) ApplyHook(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get("X-Hook-Event")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		h.responsdFail(w, http.StatusBadRequest, "read hook body failed: "+err.Error(), nil)
		return
	}
	var st hook.ServiceStateModel
	if err := json.Unmarshal(body, &st); err != nil {
		h.responsdFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
		return
	}

	// service: hook
	if err := h.hookServiceHandler.UpdateCsm(st, eventType); err != nil {
		h.responsdFail(w, http.StatusInternalServerError, "service hook failed: "+err.Error(), nil)
	}

	h.responsdSuccess(w, http.StatusOK, "hook applied", nil)
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
