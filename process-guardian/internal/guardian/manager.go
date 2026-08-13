package guardian

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"process-guardian/internal/config"
	"process-guardian/internal/types"
	"process-guardian/pkg/logger"
)

type Manager struct {
	mu              sync.RWMutex
	store           *Store
	cfg             *config.Config
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	healthDone      chan struct{}
	shutdownStarted bool
}

func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		store:      NewStore(),
		cfg:        cfg,
		ctx:        ctx,
		cancel:     cancel,
		healthDone: make(chan struct{}),
	}
}

func (m *Manager) Start() {
	logger.Info("Process Guardian Manager starting...")
	m.startHealthCheck()
	logger.Info("Process Guardian Manager started successfully")
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	if m.shutdownStarted {
		m.mu.Unlock()
		return
	}
	m.shutdownStarted = true
	m.mu.Unlock()

	logger.Info("Shutting down Process Guardian Manager...")

	m.cancel()

	select {
	case <-m.healthDone:
		logger.Info("Health check goroutine stopped")
	case <-time.After(3 * time.Second):
		logger.Warn("Health check goroutine did not stop in time")
	}

	m.stopAllProcesses()

	m.wg.Wait()

	logger.Info("Process Guardian Manager shut down")
}

func (m *Manager) RegisterProcess(req types.ProcessRequest) (*Process, error) {
	if err := m.validateRequest(req); err != nil {
		return nil, err
	}

	proc := NewProcess(req)

	if err := m.store.Add(proc); err != nil {
		return nil, err
	}

	logger.Infof("Registered process: %s (command: %s)", req.Name, req.Command)

	if err := m.startProcess(proc); err != nil {
		logger.Errorf("Failed to start process %s: %v", req.Name, err)
		return proc, err
	}

	return proc, nil
}

func (m *Manager) validateRequest(req types.ProcessRequest) error {
	if req.Name == "" {
		return fmt.Errorf("process name is required")
	}

	if req.Command == "" {
		return fmt.Errorf("command is required")
	}

	if len(req.Name) > 256 {
		return fmt.Errorf("process name too long: %d characters", len(req.Name))
	}

	if m.store.Exists(req.Name) {
		return fmt.Errorf("process %q already exists", req.Name)
	}

	return nil
}

func (m *Manager) startProcess(proc *Process) error {
	proc.SetStatus(types.StatusStarting)

	cmd := exec.Command(proc.command)
	if proc.workingDir != "" {
		cmd.Dir = proc.workingDir
	}
	if len(proc.env) > 0 {
		cmd.Env = proc.env
	}

	proc.SetCmd(cmd)

	logger.Infof("Starting process: %s (command: %s)", proc.Name(), proc.command)

	if err := cmd.Start(); err != nil {
		proc.SetStatus(types.StatusFailed)
		proc.SetLastError(err.Error())
		logger.Errorf("Failed to start process %s: %v", proc.Name(), err)
		return fmt.Errorf("failed to start process: %w", err)
	}

	proc.SetStatus(types.StatusRunning)
	proc.SetStartedAt(time.Now())
	logger.Infof("Process %s started with PID %d", proc.Name(), cmd.Process.Pid)

	m.wg.Add(1)
	go m.monitorProcess(proc)

	return nil
}

func (m *Manager) monitorProcess(proc *Process) {
	defer m.wg.Done()

	cmd := proc.Cmd()
	if cmd == nil || cmd.Process == nil {
		return
	}

	err := cmd.Wait()

	proc.SetStoppedAt(time.Now())

	if err != nil {
		logger.Warnf("Process %s exited with error: %v", proc.Name(), err)
		proc.SetStatus(types.StatusFailed)
		proc.SetLastError(err.Error())
	} else {
		logger.Infof("Process %s exited normally", proc.Name())
		proc.SetStatus(types.StatusStopped)
	}

	if proc.AutoRestart() && proc.Status() == types.StatusFailed {
		m.attemptRestart(proc)
	}
}

func (m *Manager) attemptRestart(proc *Process) {
	maxRestart := m.cfg.MaxRestartCount
	if maxRestart > 0 && proc.RestartCount() >= maxRestart {
		logger.Warnf("Process %s reached max restart count (%d), not restarting", proc.Name(), maxRestart)
		return
	}

	proc.IncrementRestartCount()
	logger.Infof("Attempting restart for process %s (attempt %d)", proc.Name(), proc.RestartCount())

	time.Sleep(m.cfg.RestartDelay)

	if err := m.startProcess(proc); err != nil {
		logger.Errorf("Failed to restart process %s: %v", proc.Name(), err)
	}
}

func (m *Manager) StopProcess(name string) error {
	proc, err := m.store.Get(name)
	if err != nil {
		return err
	}

	if !proc.IsRunning() {
		logger.Infof("Process %s is already stopped", name)
		return nil
	}

	cmd := proc.Cmd()
	if cmd == nil || cmd.Process == nil {
		logger.Warnf("Process %s has no valid command, marking as stopped", name)
		proc.SetStatus(types.StatusStopped)
		proc.SetStoppedAt(time.Now())
		return nil
	}

	logger.Infof("Stopping process %s (PID: %d)", name, cmd.Process.Pid)

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		logger.Errorf("Failed to signal process %s: %v", name, err)
		return fmt.Errorf("failed to stop process: %w", err)
	}

	proc.SetStatus(types.StatusStopped)
	proc.SetStoppedAt(time.Now())

	logger.Infof("Process %s stopped", name)
	return nil
}

func (m *Manager) StartProcess(name string) error {
	proc, err := m.store.Get(name)
	if err != nil {
		return err
	}

	if proc.IsRunning() {
		logger.Infof("Process %s is already running", name)
		return nil
	}

	logger.Infof("Manually starting process %s", name)

	if err := m.startProcess(proc); err != nil {
		return err
	}

	return nil
}

func (m *Manager) RemoveProcess(name string) error {
	proc, err := m.store.Get(name)
	if err != nil {
		return err
	}

	if proc.IsRunning() {
		if err := m.StopProcess(name); err != nil {
			return err
		}
	}

	return m.store.Remove(name)
}

func (m *Manager) GetProcess(name string) (*Process, error) {
	return m.store.Get(name)
}

func (m *Manager) ListProcesses(offset, limit int) ([]types.ProcessInfo, int) {
	return m.store.List(offset, limit)
}

func (m *Manager) ProcessCount() int {
	return m.store.Count()
}

func (m *Manager) RunningCount() int {
	return m.store.RunningCount()
}

func (m *Manager) getPid(proc *Process) int {
	cmd := proc.Cmd()
	return cmd.Process.Pid
}

func (m *Manager) startHealthCheck() {
	interval := m.cfg.HealthInterval

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(m.healthDone)

		logger.Infof("Health check goroutine started (interval: %v)", interval)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				logger.Info("Health check goroutine stopping")
				return
			case <-ticker.C:
				m.performHealthCheck()
			}
		}
	}()
}

func (m *Manager) performHealthCheck() {
	all := m.store.All()

	for name, proc := range all {
		if proc.Status() != types.StatusRunning {
			pid := m.getPid(proc)
			alive, err := checkProcessAlive(pid)
			if err != nil {
				logger.Warnf("Health check: process %s (PID: %d) check failed: %v", name, pid, err)
				continue
			}
			if alive {
				logger.Infof("Health check: process %s (PID: %d) is alive but status was %s", name, pid, proc.Status())
				proc.SetStatus(types.StatusRunning)
				proc.SetStartedAt(time.Now())
			}
			continue
		}

		pid := m.getPid(proc)
		if pid == 0 {
			continue
		}

		alive, err := checkProcessAlive(pid)
		if err != nil {
			logger.Warnf("Health check: process %s (PID: %d) check failed: %v", name, pid, err)
			continue
		}

		if !alive {
			logger.Warnf("Health check: process %s (PID: %d) is not alive", name, pid)
			proc.SetStatus(types.StatusFailed)
			proc.SetLastError("process not alive")

			if proc.AutoRestart() {
				m.attemptRestart(proc)
			}
			continue
		}

		if proc.HealthCheckCommand() != "" {
			m.runHealthCheckCommand(proc)
		}
	}
}

func (m *Manager) runHealthCheckCommand(proc *Process) {
	cmdStr := proc.HealthCheckCommand()
	cmd := exec.Command("sh", "-c", cmdStr)

	if proc.workingDir != "" {
		cmd.Dir = proc.workingDir
	}

	if err := cmd.Run(); err != nil {
		logger.Warnf("Health check command failed for %s: %v", proc.Name(), err)
	}
}

func (m *Manager) stopAllProcesses() {
	all := m.store.All()
	for name, proc := range all {
		if proc.IsRunning() {
			logger.Infof("Force stopping process %s", name)
			cmd := proc.Cmd()
			if cmd != nil && cmd.Process != nil {
				cmd.Process.Kill()
			}
			proc.SetStatus(types.StatusStopped)
			proc.SetStoppedAt(time.Now())
		}
	}
}

func checkProcessAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid PID: %d", pid)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, nil
	}

	return true, nil
}