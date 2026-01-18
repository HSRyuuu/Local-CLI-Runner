package runner

import (
	"encoding/json"
	"os/exec"
	"sync"
	"time"
)

// 상태 상수
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusStopped   = "stopped"
)

// Event는 프로세스로부터의 스트리밍 이벤트를 나타냅니다
type Event struct {
	Type      string          `json:"type"`      // stream, result, error, done
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// Result는 최종 프로세스 결과를 나타냅니다
type Result struct {
	ExitCode int             `json:"exitCode"`
	Output   json.RawMessage `json:"output,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// Process는 실행 중인 CLI 프로세스를 나타냅니다
type Process struct {
	ID          string     `json:"id"`
	Connector   string     `json:"connector"`
	Prompt      string     `json:"prompt"`
	WorkDir     string     `json:"workDir,omitempty"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// 내부
	cmd         *exec.Cmd
	events      *RingBuffer[Event]
	result      *Result
	subscribers map[string]chan Event
	cancel      func()
	mu          sync.RWMutex
	done        chan struct{}

	// result 이벤트 데이터 캐싱 (10분간 보관)
	resultData   json.RawMessage
	resultExpiry *time.Time
}

// NewProcess는 새로운 Process 인스턴스를 생성합니다
func NewProcess(id, connector, prompt, workDir string, bufferSize int) *Process {
	return &Process{
		ID:          id,
		Connector:   connector,
		Prompt:      prompt,
		WorkDir:     workDir,
		Status:      StatusPending,
		StartedAt:   time.Now(),
		events:      NewRingBuffer[Event](bufferSize),
		subscribers: make(map[string]chan Event),
		done:        make(chan struct{}),
	}
}

// AddEvent는 버퍼에 이벤트를 추가하고 모든 구독자에게 알립니다
func (p *Process) AddEvent(event Event) {
	p.mu.Lock()

	// 버퍼에 추가
	p.events.Push(event)

	// 모든 구독자에게 알림
	for _, ch := range p.subscribers {
		select {
		case ch <- event:
		default:
			// 구독자 채널이 가득 챠, 블로킹을 방지하기 위해 건너뛰기
		}
	}

	p.mu.Unlock()
}

// Subscribe는 이 구독자를 위한 새로운 이벤트 채널을 생성합니다
func (p *Process) Subscribe(subscriberID string) (<-chan Event, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 블로킹을 방지하기 위해 버퍼된 채널 생성
	ch := make(chan Event, 100)
	p.subscribers[subscriberID] = ch

	// 채널과 정리 함수 반환
	cleanup := func() {
		p.Unsubscribe(subscriberID)
	}

	return ch, cleanup
}

// Unsubscribe는 구독자를 제거합니다
func (p *Process) Unsubscribe(subscriberID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ch, exists := p.subscribers[subscriberID]; exists {
		close(ch)
		delete(p.subscribers, subscriberID)
	}
}

// GetEvents는 모든 버퍼된 이벤트를 반환합니다
func (p *Process) GetEvents() []Event {
	return p.events.ToSlice()
}

// SetStatus는 프로세스 상태를 업데이트합니다
func (p *Process) SetStatus(status string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
}

// SetResult는 최종 결과를 설정합니다
func (p *Process) SetResult(result *Result) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.result = result
}

// GetResult는 결과를 반환합니다 (완료되지 않았으면 nil)
func (p *Process) GetResult() *Result {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.result
}

// GetStatus는 현재 상태 정보를 반환합니다
func (p *Process) GetStatus() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := map[string]interface{}{
		"id":        p.ID,
		"connector": p.Connector,
		"prompt":    p.Prompt,
		"status":    p.Status,
		"startedAt": p.StartedAt,
	}

	if p.WorkDir != "" {
		status["workDir"] = p.WorkDir
	}

	if p.CompletedAt != nil {
		status["completedAt"] = p.CompletedAt
	}

	if p.result != nil {
		status["result"] = p.result
	}

	return status
}

// Stop은 실행 중인 프로세스를 종료합니다
func (p *Process) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// cancel 함수가 설정되어 있는 경우 호출 (context 취소를 트리거)
	if p.cancel != nil {
		p.cancel()
	}

	// 명령이 실행 중인 경우 종료
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}

	// done 채널 닫기
	select {
	case <-p.done:
		// 이미 닫힘
	default:
		close(p.done)
	}

	// 이미 종료 상태가 아닌 경우 상태 업데이트
	if p.Status == StatusPending || p.Status == StatusRunning {
		p.Status = StatusStopped
		now := time.Now()
		p.CompletedAt = &now
	}
}

// Close는 모든 구독자에게 알리고 정리합니다
func (p *Process) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// done 채널 닫기
	select {
	case <-p.done:
		// 이미 닫힘
	default:
		close(p.done)
	}

	// 모든 구독자 채널 닫기
	for subscriberID, ch := range p.subscribers {
		close(ch)
		delete(p.subscribers, subscriberID)
	}

	// 설정되지 않았으면 완료 시간 설정
	if p.CompletedAt == nil {
		now := time.Now()
		p.CompletedAt = &now
	}
}

// SetResultData는 result 이벤트 데이터를 저장합니다 (10분간 보관)
func (p *Process) SetResultData(data json.RawMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.resultData = data
	expiry := time.Now().Add(10 * time.Minute)
	p.resultExpiry = &expiry
}

// GetResultData는 저장된 result 데이터를 반환합니다 (만료된 경우 nil 반환)
func (p *Process) GetResultData() json.RawMessage {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 만료 확인
	if p.resultExpiry == nil || time.Now().After(*p.resultExpiry) {
		return nil
	}

	return p.resultData
}

// HasValidResultData는 유효한 result 데이터가 있는지 확인합니다
func (p *Process) HasValidResultData() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.resultData != nil && p.resultExpiry != nil && time.Now().Before(*p.resultExpiry)
}
