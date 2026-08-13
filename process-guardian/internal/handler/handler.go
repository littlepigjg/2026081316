package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"process-guardian/internal/guardian"
	"process-guardian/internal/types"
	"process-guardian/pkg/logger"
)

type Handler struct {
	manager *guardian.Manager
}

func NewHandler(manager *guardian.Manager) *Handler {
	return &Handler{manager: manager}
}

func (h *Handler) RegisterProcess(w http.ResponseWriter, r *http.Request) {
	var req types.ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("failed to parse request: %v", err))
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "name is required")
		return
	}

	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "command is required")
		return
	}

	proc, err := h.manager.RegisterProcess(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", err.Error())
		return
	}

	info := proc.Info()
	response := types.ProcessResponse{
		Name:      info.Name,
		Pid:       info.Pid,
		Status:    info.Status,
		StartedAt: info.StartedAt,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetProcess(w http.ResponseWriter, r *http.Request) {
	name := extractNameFromPath(r.URL.Path, "/process/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "process name is required")
		return
	}

	proc, err := h.manager.GetProcess(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	info := proc.Info()
	writeJSON(w, http.StatusOK, info)
}

func (h *Handler) StopProcess(w http.ResponseWriter, r *http.Request) {
	name := extractNameFromPath(r.URL.Path, "/process/")
	name = strings.TrimSuffix(name, "/stop")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "process name is required")
		return
	}

	if err := h.manager.StopProcess(name); err != nil {
		writeError(w, http.StatusInternalServerError, "stop_failed", err.Error())
		return
	}

	response := types.StopResponse{
		Name:    name,
		Stopped: true,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) StartProcess(w http.ResponseWriter, r *http.Request) {
	name := extractNameFromPath(r.URL.Path, "/process/")
	name = strings.TrimSuffix(name, "/start")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "process name is required")
		return
	}

	if err := h.manager.StartProcess(name); err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", err.Error())
		return
	}

	proc, err := h.manager.GetProcess(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	info := proc.Info()
	response := types.StartResponse{
		Name:    name,
		Started: true,
		Pid:     info.Pid,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListProcesses(w http.ResponseWriter, r *http.Request) {
	offset := 0
	limit := 10

	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			offset = parsed
		}
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	processes, total := h.manager.ListProcesses(offset, limit)

	response := types.ProcessListResponse{
		Processes: processes,
		Total:     total,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) DeleteProcess(w http.ResponseWriter, r *http.Request) {
	name := extractNameFromPath(r.URL.Path, "/process/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "process name is required")
		return
	}

	if err := h.manager.RemoveProcess(name); err != nil {
		writeError(w, http.StatusInternalServerError, "remove_failed", err.Error())
		return
	}

	response := map[string]interface{}{
		"name":     name,
		"removed":  true,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":          "ok",
		"total_processes": h.manager.ProcessCount(),
		"running":         h.manager.RunningCount(),
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("route not found: %s %s", r.Method, r.URL.Path))
}

func (h *Handler) MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", fmt.Sprintf("method %s not allowed for %s", r.Method, r.URL.Path))
}

func extractNameFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("Failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	response := types.ErrorResponse{
		Error:   errType,
		Message: message,
	}
	writeJSON(w, status, response)
}