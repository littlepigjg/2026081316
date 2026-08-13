package types

import (
	"fmt"
	"strings"
	"time"
)

type ProcessStatus string

const (
	StatusRunning  ProcessStatus = "running"
	StatusStopped  ProcessStatus = "stopped"
	StatusFailed   ProcessStatus = "failed"
	StatusStarting ProcessStatus = "starting"
)

const (
	DefaultMaxNameLength = 256
	DefaultMaxCommandLength = 4096
	DefaultHealthTimeoutSeconds = 30
	DefaultRestartDelaySeconds = 2
	DefaultMaxRestartCount = 5
	DefaultHealthIntervalSeconds = 10
	DefaultShutdownTimeoutSeconds = 5
	DefaultPort = 8080
	DefaultHost = "0.0.0.0"
)

var ValidStatuses = map[ProcessStatus]bool{
	StatusRunning:  true,
	StatusStopped:  true,
	StatusFailed:   true,
	StatusStarting: true,
}

type ProcessRequest struct {
	Name           string   `json:"name"`
	Command        string   `json:"command"`
	WorkingDir     string   `json:"working_dir,omitempty"`
	Env            []string `json:"env,omitempty"`
	AutoRestart    bool     `json:"auto_restart,omitempty"`
	HealthCheckCmd string   `json:"health_check_cmd,omitempty"`
}

func (r *ProcessRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required and cannot be empty")
	}

	if len(r.Name) > DefaultMaxNameLength {
		return fmt.Errorf("name exceeds maximum length of %d characters", DefaultMaxNameLength)
	}

	if strings.TrimSpace(r.Command) == "" {
		return fmt.Errorf("command is required and cannot be empty")
	}

	if len(r.Command) > DefaultMaxCommandLength {
		return fmt.Errorf("command exceeds maximum length of %d characters", DefaultMaxCommandLength)
	}

	for _, e := range r.Env {
		if !strings.Contains(e, "=") {
			return fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", e)
		}
	}

	return nil
}

func (r *ProcessRequest) Sanitize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Command = strings.TrimSpace(r.Command)
	r.WorkingDir = strings.TrimSpace(r.WorkingDir)
}

type ProcessInfo struct {
	Name           string        `json:"name"`
	Pid            int           `json:"pid"`
	Status         ProcessStatus `json:"status"`
	Command        string        `json:"command"`
	WorkingDir     string        `json:"working_dir,omitempty"`
	AutoRestart    bool          `json:"auto_restart"`
	HealthCheckCmd string        `json:"health_check_cmd,omitempty"`
	UptimeSeconds  int64         `json:"uptime_seconds"`
	RestartCount   int           `json:"restart_count"`
	StartedAt      time.Time     `json:"started_at"`
	StoppedAt      *time.Time    `json:"stopped_at,omitempty"`
	LastError      string        `json:"last_error,omitempty"`
}

func (p *ProcessInfo) Duration() time.Duration {
	if p.Status == StatusRunning {
		return time.Since(p.StartedAt)
	}
	if p.StoppedAt != nil {
		return p.StoppedAt.Sub(p.StartedAt)
	}
	return 0
}

func (p *ProcessInfo) IsHealthy() bool {
	return p.Status == StatusRunning && p.LastError == ""
}

type ProcessResponse struct {
	Name      string        `json:"name"`
	Pid       int           `json:"pid,omitempty"`
	Status    ProcessStatus `json:"status"`
	StartedAt time.Time     `json:"started_at,omitempty"`
}

type StopResponse struct {
	Name    string `json:"name"`
	Stopped bool   `json:"stopped"`
}

type StartResponse struct {
	Name    string `json:"name"`
	Started bool   `json:"started"`
	Pid     int    `json:"pid,omitempty"`
}

type ProcessListResponse struct {
	Processes []ProcessInfo `json:"processes"`
	Total     int           `json:"total"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type HealthStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

type ServiceStatus struct {
	Status          string    `json:"status"`
	Version         string    `json:"version"`
	TotalProcesses int       `json:"total_processes"`
	Running        int       `json:"running"`
	Stopped        int       `json:"stopped"`
	Failed         int       `json:"failed"`
	Uptime         string    `json:"uptime"`
	Timestamp      time.Time `json:"timestamp"`
}

type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Total      int         `json:"total"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	HasMore    bool        `json:"has_more"`
	NextOffset int         `json:"next_offset,omitempty"`
}

type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

type Event struct {
	Type      string    `json:"type"`
	Process   string    `json:"process"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func NewEvent(eventType, process, message string) Event {
	return Event{
		Type:      eventType,
		Process:   process,
		Message:   message,
		Timestamp: time.Now(),
	}
}

type MetricsInfo struct {
	TotalRequests    int64         `json:"total_requests"`
	TotalDuration    time.Duration `json:"total_duration"`
	AvgDurationMs    float64       `json:"avg_duration_ms"`
	Uptime           time.Duration `json:"uptime"`
	ProcessCount     int           `json:"process_count"`
	RunningCount     int           `json:"running_count"`
	FailedCount      int           `json:"failed_count"`
	RestartCount     int           `json:"restart_count"`
}

func NewMetricsInfo(totalRequests int64, totalDuration time.Duration, uptime time.Duration, processCount, runningCount, failedCount, restartCount int) MetricsInfo {
	var avgMs float64
	if totalRequests > 0 {
		avgMs = float64(totalDuration.Milliseconds()) / float64(totalRequests)
	}
	return MetricsInfo{
		TotalRequests: totalRequests,
		TotalDuration: totalDuration,
		AvgDurationMs: avgMs,
		Uptime:        uptime,
		ProcessCount:  processCount,
		RunningCount:  runningCount,
		FailedCount:   failedCount,
		RestartCount:  restartCount,
	}
}