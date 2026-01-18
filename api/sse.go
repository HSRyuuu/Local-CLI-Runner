package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"cli-runner/runner"
)

// SSEWriter는 SSE 이벤트 작성을 돕습니다
type SSEWriter struct {
	c *gin.Context
}

// NewSSEWriter는 새로운 SSE writer를 생성합니다
func NewSSEWriter(c *gin.Context) *SSEWriter {
	return &SSEWriter{c: c}
}

// SetHeaders는 SSE에 필요한 헤더를 설정합니다
func (w *SSEWriter) SetHeaders() {
	w.c.Header("Content-Type", "text/event-stream")
	w.c.Header("Cache-Control", "no-cache")
	w.c.Header("Connection", "keep-alive")
	w.c.Header("X-Accel-Buffering", "no") // Nginx 버퍼링 비활성화
}

// WriteEvent는 SSE 형식으로 단일 이벤트를 작성합니다
// 형식: event: <type>\ndata: <json>\n\n
func (w *SSEWriter) WriteEvent(event runner.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// 이벤트 타입 라인 작성
	if _, err := fmt.Fprintf(w.c.Writer, "event: %s\n", event.Type); err != nil {
		return fmt.Errorf("failed to write event type: %w", err)
	}

	// 데이터 라인 작성
	if _, err := fmt.Fprintf(w.c.Writer, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("failed to write event data: %w", err)
	}

	w.Flush()
	return nil
}

// WriteError는 에러 이벤트를 작성합니다
func (w *SSEWriter) WriteError(err error) error {
	errorData, _ := json.Marshal(map[string]string{"error": err.Error()})
	errorEvent := runner.Event{
		Type:      "error",
		Data:      errorData,
		Timestamp: time.Now(),
	}
	return w.WriteEvent(errorEvent)
}

// Flush는 응답을 플러시합니다
func (w *SSEWriter) Flush() {
	if f, ok := w.c.Writer.(http.Flusher); ok {
		f.Flush()
	}
}
