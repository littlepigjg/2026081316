package guardian

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"process-guardian/internal/types"
	"process-guardian/pkg/logger"
)

type HealthChecker struct {
	mu       sync.RWMutex
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
	manager  *Manager
	running  bool
}

func NewHealthChecker(manager *Manager, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		manager:  manager,
	}
}

func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	hc.mu.Unlock()

	go hc.run()
	logger.Infof("Health checker started with interval %v", hc.interval)
}

func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return
	}

	close(hc.stopCh)
	select {
	case <-hc.doneCh:
		logger.Info("Health checker stopped")
	case <-time.After(3 * time.Second):
		logger.Warn("Health checker stop timeout")
	}

	hc.running = false
}

func (hc *HealthChecker) IsRunning() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.running
}

func (hc *HealthChecker) run() {
	defer close(hc.doneCh)

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkAll()
		}
	}
}

func (hc *HealthChecker) checkAll() {
	processes := hc.manager.store.All()

	for name, proc := range processes {
		if proc.Status() == types.StatusRunning {
			hc.checkProcess(proc)
		} else if proc.Status() == types.StatusFailed {
			hc.checkFailedProcess(proc)
		}
		_ = name
	}
}

func (hc *HealthChecker) checkProcess(proc *Process) {
	pid := hc.manager.getPid(proc)
	if pid <= 0 {
		logger.Warnf("Health check: invalid PID for process %s", proc.Name())
		return
	}

	alive, err := verifyProcessAlive(pid)
	if err != nil {
		logger.Errorf("Health check error for %s (PID: %d): %v", proc.Name(), pid, err)
		return
	}

	if !alive {
		logger.Warnf("Health check: process %s (PID: %d) is not alive", proc.Name(), pid)
		proc.SetStatus(types.StatusFailed)
		proc.SetLastError("process liveness check failed")

		if proc.AutoRestart() {
			go hc.manager.attemptRestart(proc)
		}
		return
	}

	healthCmd := proc.HealthCheckCommand()
	if healthCmd != "" {
		hc.executeHealthCheck(proc, healthCmd)
	}
}

func (hc *HealthChecker) checkFailedProcess(proc *Process) {
	pid := hc.manager.getPid(proc)
	if pid > 0 {
		alive, err := verifyProcessAlive(pid)
		if err == nil && alive {
			logger.Infof("Health check: process %s recovered, marking as running", proc.Name())
			proc.SetStatus(types.StatusRunning)
			proc.SetStartedAt(time.Now())
			proc.SetLastError("")
		}
	}
}

func (hc *HealthChecker) executeHealthCheck(proc *Process, cmdStr string) {
	cmd := exec.Command("sh", "-c", cmdStr)

	if proc.workingDir != "" {
		cmd.Dir = proc.workingDir
	}

	if len(proc.env) > 0 {
		cmd.Env = proc.env
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warnf("Health check command failed for %s: %v (output: %s)", proc.Name(), err, string(output))
		proc.SetStatus(types.StatusFailed)
		proc.SetLastError(fmt.Sprintf("health check failed: %v", err))

		if proc.AutoRestart() {
			go hc.manager.attemptRestart(proc)
		}
		return
	}

	logger.Debugf("Health check passed for %s: %s", proc.Name(), string(output))
}

func verifyProcessAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid PID: %d", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false, nil
	}

	return true, nil
}