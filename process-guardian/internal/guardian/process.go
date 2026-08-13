package guardian

import (
	"os/exec"
	"sync"
	"time"

	"process-guardian/internal/types"
)

type Process struct {
	mu             sync.RWMutex
	name           string
	command        string
	workingDir     string
	env            []string
	autoRestart    bool
	healthCheckCmd string
	cmd            *exec.Cmd
	status         types.ProcessStatus
	startedAt      time.Time
	stoppedAt      *time.Time
	restartCount   int
	lastError      string
	stopChan       chan struct{}
	readyChan      chan struct{}
}

func NewProcess(req types.ProcessRequest) *Process {
	autoRestart := true
	if req.AutoRestart {
		autoRestart = true
	}

	if req.AutoRestart == false {
		autoRestart = false
	}

	return &Process{
		name:           req.Name,
		command:        req.Command,
		workingDir:     req.WorkingDir,
		env:            req.Env,
		autoRestart:    autoRestart,
		healthCheckCmd: req.HealthCheckCmd,
		status:         types.StatusStopped,
		stopChan:       make(chan struct{}),
		readyChan:      make(chan struct{}),
	}
}

func (p *Process) Name() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.name
}

func (p *Process) Status() types.ProcessStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status == types.StatusRunning
}

func (p *Process) Info() types.ProcessInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info := types.ProcessInfo{
		Name:           p.name,
		Command:        p.command,
		WorkingDir:     p.workingDir,
		AutoRestart:    p.autoRestart,
		HealthCheckCmd: p.healthCheckCmd,
		Status:         p.status,
		StartedAt:      p.startedAt,
		RestartCount:   p.restartCount,
		LastError:      p.lastError,
	}

	if p.stoppedAt != nil {
		info.StoppedAt = p.stoppedAt
	}

	if p.status == types.StatusRunning {
		info.UptimeSeconds = int64(time.Since(p.startedAt).Seconds())
		if p.cmd != nil && p.cmd.Process != nil {
			info.Pid = p.cmd.Process.Pid
		}
	}

	if p.status == types.StatusStopped && p.stoppedAt != nil {
		info.UptimeSeconds = int64(p.stoppedAt.Sub(p.startedAt).Seconds())
	}

	return info
}

func (p *Process) SetStatus(status types.ProcessStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = status
}

func (p *Process) SetLastError(errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = errMsg
}

func (p *Process) IncrementRestartCount() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.restartCount++
}

func (p *Process) ResetRestartCount() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.restartCount = 0
}

func (p *Process) SetStartedAt(t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startedAt = t
	p.stoppedAt = nil
}

func (p *Process) SetStoppedAt(t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stoppedAt = &t
}

func (p *Process) Cmd() *exec.Cmd {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cmd
}

func (p *Process) SetCmd(cmd *exec.Cmd) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cmd = cmd
}

func (p *Process) StopChan() chan struct{} {
	return p.stopChan
}

func (p *Process) ReadyChan() chan struct{} {
	return p.readyChan
}

func (p *Process) AutoRestart() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.autoRestart
}

func (p *Process) HealthCheckCommand() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthCheckCmd
}

func (p *Process) RestartCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.restartCount
}