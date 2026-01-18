package connector

import (
	"errors"
	"os/exec"
	"sync"

	"cli-runner/config"
	"cli-runner/runner"
)

var (
	ErrConnectorNotFound    = errors.New("connector not found")
	ErrConnectorUnavailable = errors.New("connector unavailable")
)

// Connector 인터페이스 - runner.Connector와 일치
type Connector interface {
	Name() string
	BuildCommand(prompt string) *exec.Cmd
	ParseLine(line string) (*runner.Event, error)
	IsAvailable() bool
}

// Registry는 사용 가능한 커녅터를 관리합니다
type Registry struct {
	connectors map[string]Connector
	mu         sync.RWMutex
}

// NewRegistry는 새로운 커녅터 레지스트리를 생성합니다
func NewRegistry() *Registry {
	return &Registry{
		connectors: make(map[string]Connector),
	}
}

// Register는 레지스트리에 커녅터를 추가합니다
func (r *Registry) Register(connector Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[connector.Name()] = connector
}

// Get은 이름으로 커녅터를 검색합니다
func (r *Registry) Get(name string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connector, exists := r.connectors[name]
	if !exists {
		return nil, ErrConnectorNotFound
	}

	if !connector.IsAvailable() {
		return nil, ErrConnectorUnavailable
	}

	return connector, nil
}

// List는 등록된 모든 커녅터 이름을 반환합니다
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.connectors))
	for name := range r.connectors {
		names = append(names, name)
	}
	return names
}

// Available은 사용 가능한 커녅터만 반환합니다
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.connectors))
	for name, connector := range r.connectors {
		if connector.IsAvailable() {
			names = append(names, name)
		}
	}
	return names
}

// SetupFromConfig는 설정을 기반으로 커녅터를 등록합니다
func (r *Registry) SetupFromConfig(cfg *config.Config) {
	// Claude 커녅터 등록
	r.Register(NewClaudeConnector(cfg.Connectors.Claude))
}
