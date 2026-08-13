package guardian

import (
	"fmt"
	"sync"
	"time"

	"process-guardian/internal/types"
)

type Store struct {
	mu        sync.RWMutex
	processes map[string]*Process
	createdAt time.Time
	updatedAt time.Time
}

func NewStore() *Store {
	now := time.Now()
	return &Store{
		processes: make(map[string]*Process),
		createdAt: now,
		updatedAt: now,
	}
}

func (s *Store) Add(proc *Process) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.processes[proc.Name()]; exists {
		return fmt.Errorf("process with name %q already exists", proc.Name())
	}

	s.processes[proc.Name()] = proc
	s.updatedAt = time.Now()
	return nil
}

func (s *Store) Get(name string) (*Process, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	proc, exists := s.processes[name]
	if !exists {
		return nil, fmt.Errorf("process with name %q not found", name)
	}

	return proc, nil
}

func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.processes[name]; !exists {
		return fmt.Errorf("process with name %q not found", name)
	}

	delete(s.processes, name)
	s.updatedAt = time.Now()
	return nil
}

func (s *Store) List(offset, limit int) ([]types.ProcessInfo, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allProcs := make([]types.ProcessInfo, 0, len(s.processes))
	for _, proc := range s.processes {
		allProcs = append(allProcs, proc.Info())
	}

	total := len(allProcs)

	if offset >= total {
		return []types.ProcessInfo{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	if limit <= 0 {
		limit = 10
	}

	return allProcs[offset:end], total
}

func (s *Store) All() map[string]*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*Process, len(s.processes))
	for k, v := range s.processes {
		result[k] = v
	}
	return result
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.processes)
}

func (s *Store) RunningCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, proc := range s.processes {
		if proc.IsRunning() {
			count++
		}
	}
	return count
}

func (s *Store) StoppedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, proc := range s.processes {
		if !proc.IsRunning() {
			count++
		}
	}
	return count
}

func (s *Store) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.processes))
	for name := range s.processes {
		names = append(names, name)
	}
	return names
}

func (s *Store) Exists(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.processes[name]
	return exists
}

func (s *Store) UpdateTimestamp() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updatedAt = time.Now()
}

func (s *Store) CreatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.createdAt
}

func (s *Store) UpdatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}