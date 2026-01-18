package runner

import (
	"errors"
	"sync"
	"time"

	"cli-runner/config"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var (
	ErrProcessNotFound = errors.New("process not found")
	ErrMaxConcurrent   = errors.New("max concurrent processes reached")
)

// Manager는 모든 실행 중인 프로세스를 관리합니다
type Manager struct {
	processes map[string]*Process
	config    *config.Config
	logger    zerolog.Logger
	mu        sync.RWMutex
}

// NewManager는 새로운 ProcessManager를 생성합니다
func NewManager(cfg *config.Config, logger zerolog.Logger) *Manager {
	return &Manager{
		processes: make(map[string]*Process),
		config:    cfg,
		logger:    logger.With().Str("component", "manager").Logger(),
	}
}

// Create는 새로운 프로세스를 생성합니다 (상태: pending)
func (m *Manager) Create(connector, prompt, workDir string) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 최대 동시 실행 제한 확인
	activeCount := 0
	for _, p := range m.processes {
		if p.Status == StatusPending || p.Status == StatusRunning {
			activeCount++
		}
	}

	if activeCount >= m.config.Process.MaxConcurrent {
		m.logger.Warn().
			Int("active", activeCount).
			Int("max", m.config.Process.MaxConcurrent).
			Msg("Max concurrent processes reached")
		return nil, ErrMaxConcurrent
	}

	// 프로세스 ID를 위한 UUID 생성
	id := uuid.New().String()

	// 새 프로세스 생성
	process := NewProcess(id, connector, prompt, workDir, m.config.Process.BufferSize)

	// 프로세스 등록
	m.processes[id] = process

	m.logger.Info().
		Str("processId", id).
		Str("connector", connector).
		Str("workDir", workDir).
		Msg("Process created")

	return process, nil
}

// Get은 ID로 프로세스를 검색합니다
func (m *Manager) Get(id string) (*Process, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	process, exists := m.processes[id]
	if !exists {
		return nil, ErrProcessNotFound
	}

	return process, nil
}

// List는 모든 프로세스를 반환합니다
func (m *Manager) List() []*Process {
	m.mu.RLock()
	defer m.mu.RUnlock()

	processes := make([]*Process, 0, len(m.processes))
	for _, p := range m.processes {
		processes = append(processes, p)
	}

	return processes
}

// Stop은 프로세스를 종료합니다
func (m *Manager) Stop(id string) error {
	m.mu.RLock()
	process, exists := m.processes[id]
	m.mu.RUnlock()

	if !exists {
		return ErrProcessNotFound
	}

	// 프로세스 중지 (Process.Stop()이 실제 종료를 처리해야 함)
	process.Stop()

	m.logger.Info().
		Str("processId", id).
		Msg("Process stopped")

	return nil
}

// Remove는 완료된 프로세스를 제거합니다
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, exists := m.processes[id]
	if !exists {
		return ErrProcessNotFound
	}

	// completed, failed, 또는 stopped 프로세스만 제거
	if process.Status != StatusCompleted &&
		process.Status != StatusFailed &&
		process.Status != StatusStopped {
		m.logger.Warn().
			Str("processId", id).
			Str("status", process.Status).
			Msg("Cannot remove active process")
		return errors.New("cannot remove active process")
	}

	delete(m.processes, id)

	m.logger.Info().
		Str("processId", id).
		Msg("Process removed")

	return nil
}

// Count는 활성 프로세스의 수를 반환합니다
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeCount := 0
	for _, p := range m.processes {
		if p.Status == StatusPending || p.Status == StatusRunning {
			activeCount++
		}
	}

	return activeCount
}

// StartCleanup은 오래된 완료된 프로세스를 정리하는 고루틴을 시작합니다
func (m *Manager) StartCleanup() {
	go func() {
		ticker := time.NewTicker(m.config.Process.CleanupDelay)
		defer ticker.Stop()

		m.logger.Info().
			Dur("interval", m.config.Process.CleanupDelay).
			Msg("Process cleanup started")

		for range ticker.C {
			m.cleanup()
		}
	}()
}

// cleanup은 오래된 완료된 프로세스를 제거합니다
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cleanupThreshold := m.config.Process.CleanupDelay
	removed := 0

	for id, process := range m.processes {
		// 프로세스가 종료 상태인지 확인
		if process.Status != StatusCompleted &&
			process.Status != StatusFailed &&
			process.Status != StatusStopped {
			continue
		}

		// 완료 시간이 설정되어 있고 충분히 오래되었는지 확인
		if process.CompletedAt != nil {
			age := now.Sub(*process.CompletedAt)
			if age >= cleanupThreshold {
				delete(m.processes, id)
				removed++

				m.logger.Debug().
					Str("processId", id).
					Str("status", process.Status).
					Dur("age", age).
					Msg("Process cleaned up")
			}
		}
	}

	if removed > 0 {
		m.logger.Info().
			Int("removed", removed).
			Int("remaining", len(m.processes)).
			Msg("Cleanup completed")
	}
}
