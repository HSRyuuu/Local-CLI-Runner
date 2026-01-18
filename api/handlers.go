package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"cli-runner/connector"
	"cli-runner/runner"
)

// Handlers는 모든 HTTP 핸들러를 포함합니다
type Handlers struct {
	manager  *runner.Manager
	runner   *runner.Runner
	registry *connector.Registry
	logger   zerolog.Logger
}

// NewHandlers는 의존성과 함께 핸들러를 생성합니다
func NewHandlers(manager *runner.Manager, runnerInstance *runner.Runner, registry *connector.Registry, logger zerolog.Logger) *Handlers {
	return &Handlers{
		manager:  manager,
		runner:   runnerInstance,
		registry: registry,
		logger:   logger.With().Str("component", "handlers").Logger(),
	}
}

// RunRequest는 POST /run 요청 바디를 나타냅니다
type RunRequest struct {
	Connector string `json:"connector" binding:"required" example:"claude"`
	Prompt    string `json:"prompt" binding:"required" example:"Hello, how are you?"`
	WorkDir   string `json:"workDir,omitempty" example:"/path/to/project"`
}

// RunResponse는 POST /run의 응답을 나타냅니다
type RunResponse struct {
	ProcessID string `json:"processId" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// ErrorResponse는 에러 응답을 나타냅니다
type ErrorResponse struct {
	Error   string `json:"error" example:"Invalid request body"`
	Details string `json:"details,omitempty" example:"connector is required"`
}

// ProcessStatus는 프로세스의 상태를 나타냅니다
type ProcessStatus struct {
	ID          string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Connector   string  `json:"connector" example:"claude"`
	Prompt      string  `json:"prompt" example:"Hello"`
	WorkDir     string  `json:"workDir,omitempty" example:"/path/to/project"`
	Status      string  `json:"status" example:"running"`
	StartedAt   string  `json:"startedAt" example:"2024-01-01T12:00:00Z"`
	CompletedAt *string `json:"completedAt,omitempty" example:"2024-01-01T12:01:00Z"`
}

// ProcessResult는 완료된 프로세스의 결과를 나타냅니다
type ProcessResult struct {
	ExitCode int    `json:"exitCode" example:"0"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ProcessListResponse는 프로세스 목록을 나타냅니다
type ProcessListResponse struct {
	Processes []ProcessStatus `json:"processes"`
	Count     int             `json:"count" example:"5"`
}

// ConnectorListResponse는 커넥터 목록을 나타냅니다
type ConnectorListResponse struct {
	Connectors []string `json:"connectors" example:"claude,gemini"`
	Count      int      `json:"count" example:"2"`
}

// MessageResponse는 간단한 메시지 응답을 나타냅니다
type MessageResponse struct {
	Message string `json:"message" example:"Process deleted successfully"`
}

// RunHandler handles POST /api/v1/run
// @Summary 프로세스 실행
// @Description AI CLI 프로세스를 실행하고 processId를 반환합니다
// @Tags process
// @Accept json
// @Produce json
// @Param request body RunRequest true "실행 요청"
// @Success 202 {object} RunResponse "프로세스가 생성됨"
// @Failure 400 {object} ErrorResponse "잘못된 요청"
// @Failure 429 {object} ErrorResponse "최대 동시 실행 수 초과"
// @Failure 500 {object} ErrorResponse "서버 오류"
// @Router /run [post]
func (h *Handlers) RunHandler(c *gin.Context) {
	var req RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// 레지스트리에서 커넥터 가져오기
	conn, err := h.registry.Get(req.Connector)
	if err != nil {
		h.logger.Warn().
			Str("connector", req.Connector).
			Err(err).
			Msg("Connector not found or unavailable")
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Connector '%s' not found or unavailable", req.Connector)})
		return
	}

	// 매니저를 통해 프로세스 생성
	process, err := h.manager.Create(req.Connector, req.Prompt, req.WorkDir)
	if err != nil {
		if err == runner.ErrMaxConcurrent {
			h.logger.Warn().Msg("Max concurrent processes reached")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Maximum concurrent processes reached"})
			return
		}
		h.logger.Error().Err(err).Msg("Failed to create process")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create process"})
		return
	}

	// 러너를 통해 실행 생성 (30분 타임아웃)
	timeout := 30 * time.Minute
	if err := h.runner.Spawn(process, conn, timeout); err != nil {
		h.logger.Error().
			Str("processId", process.ID).
			Err(err).
			Msg("Failed to spawn process")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to spawn process"})
		return
	}

	h.logger.Info().
		Str("processId", process.ID).
		Str("connector", req.Connector).
		Msg("Process spawned successfully")

	// 즉시 processId 반환
	c.JSON(http.StatusAccepted, gin.H{"processId": process.ID})
}

// StreamHandler handles GET /api/v1/stream/:id
// @Summary SSE 스트림 구독
// @Description 프로세스의 실시간 이벤트를 SSE로 스트리밍합니다
// @Tags stream
// @Produce text/event-stream
// @Param id path string true "프로세스 ID"
// @Success 200 {string} string "SSE 이벤트 스트림"
// @Failure 404 {object} ErrorResponse "프로세스를 찾을 수 없음"
// @Router /stream/{id} [get]
func (h *Handlers) StreamHandler(c *gin.Context) {
	processID := c.Param("id")

	// 프로세스 가져오기
	process, err := h.manager.Get(processID)
	if err != nil {
		h.logger.Warn().
			Str("processId", processID).
			Msg("Process not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Process not found"})
		return
	}

	// SSE 헤더 설정
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	h.logger.Info().
		Str("processId", processID).
		Msg("SSE stream started")

	// 먼저 버퍼된 이벤트 전송
	bufferedEvents := process.GetEvents()
	for _, event := range bufferedEvents {
		h.writeSSEEvent(c.Writer, event)
		c.Writer.Flush()
	}

	// 실시간 이벤트 구독
	subscriberID := uuid.New().String()
	eventChan, cleanup := process.Subscribe(subscriberID)
	defer cleanup()

	// 완료되거나 클라이언트가 연결을 끊을 때까지 실시간 이벤트 스트리밍
	clientClosed := c.Request.Context().Done()

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// 채널이 닫힘, 프로세스 완료
				h.logger.Info().
					Str("processId", processID).
					Msg("Event channel closed")
				return
			}

			// 응답에 이벤트 작성
			if err := h.writeSSEEvent(c.Writer, event); err != nil {
				h.logger.Warn().
					Str("processId", processID).
					Err(err).
					Msg("Failed to write SSE event")
				return
			}
			c.Writer.Flush()

			// done 이벤트인 경우 스트림 닫기
			if event.Type == "done" {
				h.logger.Info().
					Str("processId", processID).
					Msg("Done event received, closing stream")
				return
			}

		case <-clientClosed:
			// 클라이언트 연결 끊김
			h.logger.Info().
				Str("processId", processID).
				Msg("Client disconnected from SSE stream")
			return
		}
	}
}

// GetProcessHandler handles GET /api/v1/process/:id
// @Summary 프로세스 상태 조회
// @Description 특정 프로세스의 상태를 조회합니다
// @Tags process
// @Produce json
// @Param id path string true "프로세스 ID"
// @Success 200 {object} ProcessStatus "프로세스 상태"
// @Failure 404 {object} ErrorResponse "프로세스를 찾을 수 없음"
// @Router /process/{id} [get]
func (h *Handlers) GetProcessHandler(c *gin.Context) {
	processID := c.Param("id")

	// 프로세스 가져오기
	process, err := h.manager.Get(processID)
	if err != nil {
		h.logger.Warn().
			Str("processId", processID).
			Msg("Process not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Process not found"})
		return
	}

	// 프로세스 상태 반환
	c.JSON(http.StatusOK, process.GetStatus())
}

// GetResultHandler handles GET /api/v1/result/:id
// @Summary 프로세스 결과 조회
// @Description 완료된 프로세스의 결과를 조회합니다
// @Tags process
// @Produce json
// @Param id path string true "프로세스 ID"
// @Success 200 {object} ProcessResult "프로세스 결과"
// @Success 202 {object} map[string]string "프로세스 실행 중"
// @Failure 404 {object} ErrorResponse "프로세스를 찾을 수 없음"
// @Router /result/{id} [get]
func (h *Handlers) GetResultHandler(c *gin.Context) {
	processID := c.Param("id")

	// 프로세스 가져오기
	process, err := h.manager.Get(processID)
	if err != nil {
		h.logger.Warn().
			Str("processId", processID).
			Msg("Process not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Process not found"})
		return
	}

	// 결과 가져오기
	result := process.GetResult()
	if result == nil {
		// 아직 완료되지 않음, 상태와 함께 202 Accepted 반환
		h.logger.Debug().
			Str("processId", processID).
			Msg("Process not yet completed")
		c.JSON(http.StatusAccepted, gin.H{
			"status":  process.Status,
			"message": "Process is still running",
		})
		return
	}

	// 결과 반환
	c.JSON(http.StatusOK, result)
}

// DeleteProcessHandler handles DELETE /api/v1/process/:id
// @Summary 프로세스 종료 및 삭제
// @Description 실행 중인 프로세스를 종료하고 삭제합니다
// @Tags process
// @Produce json
// @Param id path string true "프로세스 ID"
// @Success 200 {object} MessageResponse "삭제 성공"
// @Failure 400 {object} ErrorResponse "삭제 실패"
// @Failure 404 {object} ErrorResponse "프로세스를 찾을 수 없음"
// @Router /process/{id} [delete]
func (h *Handlers) DeleteProcessHandler(c *gin.Context) {
	processID := c.Param("id")

	// 프로세스 중지
	if err := h.manager.Stop(processID); err != nil {
		h.logger.Warn().
			Str("processId", processID).
			Err(err).
			Msg("Failed to stop process")
		c.JSON(http.StatusNotFound, gin.H{"error": "Process not found"})
		return
	}

	// 프로세스 제거
	if err := h.manager.Remove(processID); err != nil {
		h.logger.Warn().
			Str("processId", processID).
			Err(err).
			Msg("Failed to remove process")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info().
		Str("processId", processID).
		Msg("Process stopped and removed")

	c.JSON(http.StatusOK, gin.H{"message": "Process deleted successfully"})
}

// ListProcessesHandler handles GET /api/v1/processes
// @Summary 프로세스 목록 조회
// @Description 모든 프로세스의 목록을 조회합니다
// @Tags process
// @Produce json
// @Success 200 {object} ProcessListResponse "프로세스 목록"
// @Router /processes [get]
func (h *Handlers) ListProcessesHandler(c *gin.Context) {
	processes := h.manager.List()

	// 상태 객체로 변환
	result := make([]map[string]interface{}, 0, len(processes))
	for _, process := range processes {
		result = append(result, process.GetStatus())
	}

	c.JSON(http.StatusOK, gin.H{
		"processes": result,
		"count":     len(result),
	})
}

// ListConnectorsHandler handles GET /api/v1/connectors
// @Summary 사용 가능한 커넥터 목록
// @Description 사용 가능한 AI CLI 커넥터 목록을 조회합니다
// @Tags connector
// @Produce json
// @Success 200 {object} ConnectorListResponse "커넥터 목록"
// @Router /connectors [get]
func (h *Handlers) ListConnectorsHandler(c *gin.Context) {
	connectors := h.registry.Available()

	c.JSON(http.StatusOK, gin.H{
		"connectors": connectors,
		"count":      len(connectors),
	})
}

// GetResultDataHandler handles GET /api/v1/result-data/:id
// @Summary 캐시된 result 데이터 조회
// @Description 10분간 메모리에 저장된 result 이벤트 데이터를 조회합니다
// @Tags process
// @Produce json
// @Param id path string true "프로세스 ID"
// @Success 200 {object} map[string]interface{} "Result 데이터"
// @Failure 404 {object} ErrorResponse "프로세스를 찾을 수 없거나 데이터가 만료됨"
// @Router /result-data/{id} [get]
func (h *Handlers) GetResultDataHandler(c *gin.Context) {
	processID := c.Param("id")

	// 프로세스 가져오기
	process, err := h.manager.Get(processID)
	if err != nil {
		h.logger.Warn().
			Str("processId", processID).
			Msg("Process not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Process not found"})
		return
	}

	// result 데이터 가져오기
	resultData := process.GetResultData()
	if resultData == nil {
		h.logger.Debug().
			Str("processId", processID).
			Msg("Result data not found or expired")
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Result data not found or expired",
			"message": "Result data is only cached for 10 minutes",
		})
		return
	}

	h.logger.Info().
		Str("processId", processID).
		Int("dataSize", len(resultData)).
		Msg("Result data retrieved from cache")

	// JSON으로 파싱하여 반환
	var resultJSON map[string]interface{}
	if err := json.Unmarshal(resultData, &resultJSON); err != nil {
		h.logger.Error().
			Str("processId", processID).
			Err(err).
			Msg("Failed to parse result data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result data"})
		return
	}

	c.JSON(http.StatusOK, resultJSON)
}


// writeSSEEvent는 SSE 형식으로 이벤트를 작성합니다
func (h *Handlers) writeSSEEvent(w io.Writer, event runner.Event) error {
	// 형식: event: <type>\ndata: <json>\n\n
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(event.Data))
	return err
}
