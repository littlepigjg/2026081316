package guardian

import (
	"sync"
	"time"

	"process-guardian/internal/config"
	"process-guardian/internal/types"
	"process-guardian/pkg/logger"
)

type RestartPolicy struct {
	mu             sync.Mutex
	maxRestarts    int
	restartDelay   time.Duration
	backoffFactor  float64
	restartCounts  map[string]int
	lastRestartAt  map[string]time.Time
	cooldownPeriod time.Duration
}

func NewRestartPolicy(cfg *config.Config) *RestartPolicy {
	return &RestartPolicy{
		maxRestarts:    cfg.MaxRestartCount,
		restartDelay:   cfg.RestartDelay,
		backoffFactor:  2.0,
		restartCounts:  make(map[string]int),
		lastRestartAt:  make(map[string]time.Time),
		cooldownPeriod: 30 * time.Second,
	}
}

func (rp *RestartPolicy) CanRestart(name string) bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	count := rp.restartCounts[name]
	if rp.maxRestarts > 0 && count >= rp.maxRestarts {
		return false
	}

	if lastTime, exists := rp.lastRestartAt[name]; exists {
		if time.Since(lastTime) < rp.cooldownPeriod {
			return true
		}
	}

	return true
}

func (rp *RestartPolicy) NextDelay(name string) time.Duration {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	count := rp.restartCounts[name]
	delay := rp.restartDelay
	for i := 0; i < count; i++ {
		delay = time.Duration(float64(delay) * rp.backoffFactor)
	}

	maxDelay := 60 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

func (rp *RestartPolicy) RecordRestart(name string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	rp.restartCounts[name]++
	rp.lastRestartAt[name] = time.Now()
}

func (rp *RestartPolicy) ResetCount(name string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	delete(rp.restartCounts, name)
	delete(rp.lastRestartAt, name)
}

func (rp *RestartPolicy) GetCount(name string) int {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.restartCounts[name]
}

func (rp *RestartPolicy) ShouldReset(name string) bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	lastTime, exists := rp.lastRestartAt[name]
	if !exists {
		return true
	}

	if time.Since(lastTime) > rp.cooldownPeriod*2 {
		return true
	}

	return false
}

type RestartManager struct {
	policy  *RestartPolicy
	manager *Manager
	wg      sync.WaitGroup
	stopCh  chan struct{}
}

func NewRestartManager(manager *Manager, policy *RestartPolicy) *RestartManager {
	return &RestartManager{
		policy:  policy,
		manager: manager,
		stopCh:  make(chan struct{}),
	}
}

func (rm *RestartManager) Start() {
	rm.wg.Add(1)
	go rm.run()
	logger.Info("Restart manager started")
}

func (rm *RestartManager) Stop() {
	close(rm.stopCh)
	rm.wg.Wait()
	logger.Info("Restart manager stopped")
}

func (rm *RestartManager) run() {
	defer rm.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.stopCh:
			return
		case <-ticker.C:
			rm.checkAndRestart()
		}
	}
}

func (rm *RestartManager) checkAndRestart() {
	processes := rm.manager.store.All()

	for name, proc := range processes {
		if proc.IsRunning() {
			if rm.policy.ShouldReset(name) {
				rm.policy.ResetCount(name)
			}
			continue
		}

		if proc.Status() == types.StatusFailed && proc.AutoRestart() {
			if rm.policy.CanRestart(name) {
				delay := rm.policy.NextDelay(name)
				logger.Infof("Scheduling restart for %s in %v", name, delay)

				rm.policy.RecordRestart(name)

				go func(p *Process, d time.Duration) {
					time.Sleep(d)
					if err := rm.manager.startProcess(p); err != nil {
						logger.Errorf("Failed to restart %s: %v", p.Name(), err)
					}
				}(proc, delay)
			} else {
				logger.Warnf("Max restart count reached for %s, not restarting", name)
			}
		}
	}
}

func (rm *RestartManager) RestartNow(name string) error {
	proc, err := rm.manager.store.Get(name)
	if err != nil {
		return err
	}

	if proc.IsRunning() {
		return nil
	}

	rm.policy.ResetCount(name)

	return rm.manager.startProcess(proc)
}