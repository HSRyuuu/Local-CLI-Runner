package connector

import (
	"encoding/json"
	"os/exec"
	"strings"

	"cli-runner/config"
	"cli-runner/runner"
)

// ClaudeConnector는 Claude CLI를 위한 Connector를 구현합니다
type ClaudeConnector struct {
	config config.ConnectorConfig
}

// NewClaudeConnector는 새로운 Claude 커녅터를 생성합니다
func NewClaudeConnector(cfg config.ConnectorConfig) *ClaudeConnector {
	return &ClaudeConnector{
		config: cfg,
	}
}

// Name은 커녅터 이름을 반환합니다
func (c *ClaudeConnector) Name() string {
	return "claude"
}

// IsAvailable은 커녅터를 사용할 수 있는지 여부를 반환합니다
func (c *ClaudeConnector) IsAvailable() bool {
	return c.config.Available
}

// BuildCommand는 실행할 명령을 구축합니다
func (c *ClaudeConnector) BuildCommand(prompt string) *exec.Cmd {
	// 구축: claude [설정의 args] -p "prompt"
	args := append(c.config.Args, "-p", prompt)
	return exec.Command(c.config.Command, args...)
}

// ParseLine은 Claude CLI 출력에서 JSON 라인을 파싱합니다
func (c *ClaudeConnector) ParseLine(line string) (*runner.Event, error) {
	// 빈 라인 건너뛰기
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	// JSON 라인 파싱
	var rawJSON map[string]interface{}
	if err := json.Unmarshal([]byte(line), &rawJSON); err != nil {
		// 유효한 JSON이 아님, 건너뛰기
		return nil, nil
	}

	// "type" 필드를 기반으로 이벤트 타입 결정
	eventType := "stream" // 기본값은 stream
	if typeField, ok := rawJSON["type"].(string); ok {
		if typeField == "result" {
			eventType = "result"
		}
	}

	// 원본 JSON을 Data로 보존
	data := json.RawMessage(line)

	event := &runner.Event{
		Type: eventType,
		Data: data,
	}

	return event, nil
}
