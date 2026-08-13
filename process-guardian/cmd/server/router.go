package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"process-guardian/internal/guardian"
	"process-guardian/internal/handler"
	"process-guardian/internal/middleware"
	"process-guardian/pkg/logger"
)

type Router struct {
	handler   *handler.Handler
	manager   *guardian.Manager
	mux       *http.ServeMux
	collector *middleware.MetricsCollector
}

func NewRouter(manager *guardian.Manager) *Router {
	return &Router{
		handler:   handler.NewHandler(manager),
		manager:   manager,
		mux:       http.NewServeMux(),
		collector: middleware.NewMetricsCollector(),
	}
}

func (rt *Router) SetupRoutes() {
	inner := rt.setupRoutes()

	var h http.Handler = inner
	h = middleware.RecoveryMiddleware(h)
	h = middleware.CORSMiddleware(h)
	h = middleware.ContentTypeMiddleware(h)
	h = middleware.RequestIDMiddleware(h)
	h = middleware.MetricsMiddleware(rt.collector)(h)
	h = middleware.LoggingMiddleware(h)

	rt.mux.Handle("/", h)
}

func (rt *Router) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", rt.handleHealth)
	mux.HandleFunc("/status", rt.handler.GetStatus)
	mux.HandleFunc("/processes", rt.handleProcesses)
	mux.HandleFunc("/process", rt.handleProcess)
	mux.HandleFunc("/process/", rt.handleProcessByName)

	return mux
}

func (rt *Router) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		rt.handler.MethodNotAllowedHandler(w, r)
		return
	}

	response := map[string]interface{}{
		"status": "ok",
		"uptime": "service is running",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (rt *Router) handleProcesses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rt.handler.ListProcesses(w, r)
	default:
		rt.handler.MethodNotAllowedHandler(w, r)
	}
}

func (rt *Router) handleProcess(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		rt.handler.RegisterProcess(w, r)
	default:
		rt.handler.MethodNotAllowedHandler(w, r)
	}
}

func (rt *Router) handleProcessByName(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if strings.HasSuffix(path, "/stop") {
		if r.Method == http.MethodPost {
			rt.handler.StopProcess(w, r)
			return
		}
		rt.handler.MethodNotAllowedHandler(w, r)
		return
	}

	if strings.HasSuffix(path, "/start") {
		if r.Method == http.MethodPost {
			rt.handler.StartProcess(w, r)
			return
		}
		rt.handler.MethodNotAllowedHandler(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rt.handler.GetProcess(w, r)
	case http.MethodDelete:
		rt.handler.DeleteProcess(w, r)
	default:
		rt.handler.MethodNotAllowedHandler(w, r)
	}
}

func (rt *Router) Handler() http.Handler {
	return rt.mux
}

func (rt *Router) MetricsCollector() *middleware.MetricsCollector {
	return rt.collector
}

func (rt *Router) logRequest(method, path string, status int) {
	logger.Debug("Request: %s %s -> %d", method, path, status)
}