package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/rs/zerolog"
)

// Connector는 다양한 CLI 도구를 위한 인터페이스입니다
type Connector interface {
	Name() string
	BuildCommand(prompt string) *exec.Cmd
	ParseLine(line string) (*Event, error)
}

// Runner는 프로세스 실행을 처리합니다
type Runner struct {
	manager *Manager
	logger  zerolog.Logger
}

// NewRunner는 새로운 Runner를 생성합니다
func NewRunner(manager *Manager, logger zerolog.Logger) *Runner {
	return &Runner{
		manager: manager,
		logger:  logger.With().Str("component", "runner").Logger(),
	}
}

// Spawn은 고루틴에서 프로세스 실행을 시작합니다
// 시작 후 즉시 반환합니다
func (r *Runner) Spawn(process *Process, connector Connector, timeout time.Duration) error {
	if process == nil {
		return fmt.Errorf("process cannot be nil")
	}
	if connector == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	r.logger.Info().
		Str("processId", process.ID).
		Str("connector", connector.Name()).
		Dur("timeout", timeout).
		Msg("Spawning process")

	// 초기 상태를 running으로 설정
	process.SetStatus(StatusRunning)

	// 타임아웃으로 context 생성
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// Stop() 지원을 위해 프로세스에 cancel 함수 저장
	process.mu.Lock()
	process.cancel = cancel
	process.mu.Unlock()

	// 고루틴에서 실행 시작
	go func() {
		defer cancel()
		r.run(ctx, process, connector)
	}()

	return nil
}

// run은 프로세스를 실행합니다 (Spawn에서 고루틴에서 호출됨)
func (r *Runner) run(ctx context.Context, process *Process, connector Connector) {
	startTime := time.Now()

	r.logger.Info().
		Str("processId", process.ID).
		Str("connector", connector.Name()).
		Msg("Running process")

	// 명령 구축
	cmd := connector.BuildCommand(process.Prompt)
	cmd.Stderr = cmd.Stdout // 통합 스트리밍을 위해 stderr를 stdout에 병합

	// working directory 설정
	if process.WorkDir != "" {
		cmd.Dir = process.WorkDir
	}

	// 명령 참조 저장
	process.mu.Lock()
	process.cmd = cmd
	process.mu.Unlock()

	// stdout를 위한 파이프 생성
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.handleError(process, fmt.Errorf("failed to create stdout pipe: %w", err))
		return
	}

	// 명령 시작
	if err := cmd.Start(); err != nil {
		r.handleError(process, fmt.Errorf("failed to start command: %w", err))
		return
	}

	// CLI 명령 시작 로그 (상세)
	r.logger.Info().
		Str("processId", process.ID).
		Str("connector", connector.Name()).
		Int("pid", cmd.Process.Pid).
		Str("command", cmd.Path).
		Strs("args", cmd.Args).
		Str("workDir", process.WorkDir).
		Str("prompt", process.Prompt).
		Msg("CLI process started")

	// 별도의 고루틴에서 출력 스트리밍
	streamDone := make(chan struct{})
	go func() {
		r.streamOutput(stdout, process, connector)
		close(streamDone)
	}()

	// 명령 완료 또는 context 취소를 대기
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	var cmdErr error
	select {
	case <-ctx.Done():
		// Context가 취소됨 (타임아웃 또는 수동 중지)
		r.logger.Warn().
			Str("processId", process.ID).
			Err(ctx.Err()).
			Msg("Process context cancelled")

		// 프로세스 종료
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				r.logger.Error().
					Str("processId", process.ID).
					Err(err).
					Msg("Failed to kill process")
			}
		}

		// 명령이 종료되기를 대기
		<-cmdDone
		process.SetStatus(StatusStopped)

		// 에러 이벤트 전송
		r.sendErrorEvent(process, "Process stopped or timed out")

	case cmdErr = <-cmdDone:
		// 명령이 정상적으로 완료됨
		<-streamDone // 스트림이 종료되기를 대기

		if cmdErr != nil {
			duration := time.Since(startTime)
			exitCode := getExitCode(cmdErr)

			// CLI 명령 실패 로그 (상세)
			r.logger.Error().
				Str("processId", process.ID).
				Str("connector", connector.Name()).
				Err(cmdErr).
				Int("exitCode", exitCode).
				Dur("duration", duration).
				Str("prompt", process.Prompt).
				Msg("CLI process failed")

			process.SetStatus(StatusFailed)

			// 결과 설정
			result := &Result{
				ExitCode: exitCode,
				Error:    cmdErr.Error(),
			}
			process.SetResult(result)

			// 에러 이벤트 전송
			r.sendErrorEvent(process, cmdErr.Error())
		} else {
			duration := time.Since(startTime)

			// CLI 명령 성공 로그 (상세)
			r.logger.Info().
				Str("processId", process.ID).
				Str("connector", connector.Name()).
				Int("exitCode", 0).
				Dur("duration", duration).
				Str("prompt", process.Prompt).
				Msg("CLI process completed successfully")

			process.SetStatus(StatusCompleted)

			// 결과 설정
			result := &Result{
				ExitCode: 0,
			}
			process.SetResult(result)
		}
	}

	// done 이벤트 전송
	r.sendDoneEvent(process)

	// 프로세스 닫고 구독자 정리
	process.Close()

	// 최종 실행 종료 로그
	duration := time.Since(startTime)
	result := process.GetResult()

	logEvent := r.logger.Info().
		Str("processId", process.ID).
		Str("connector", connector.Name()).
		Str("status", process.Status).
		Dur("totalDuration", duration)

	if result != nil {
		logEvent = logEvent.Int("exitCode", result.ExitCode)
		if result.Error != "" {
			logEvent = logEvent.Str("error", result.Error)
		}
	}

	logEvent.Msg("CLI process execution finished")
}

// streamOutput은 리더로부터 읽고 이벤트를 전송합니다
func (r *Runner) streamOutput(reader io.Reader, process *Process, connector Connector) {
	scanner := bufio.NewScanner(reader)

	// 긴 라인을 위해 더 큰 버퍼 크기 설정 (기본값은 64KB, 필요한 경우 증가)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()

		// 커녅터를 사용하여 라인 파싱
		event, err := connector.ParseLine(line)
		if err != nil {
			r.logger.Warn().
				Str("processId", process.ID).
				Err(err).
				Str("line", line).
				Msg("Failed to parse line")
			continue
		}

		// nil 이벤트 건너뛰기 (커녅터가 이 라인을 무시하기로 결정)
		if event == nil {
			continue
		}

		// 타임스탬프 설정
		event.Timestamp = time.Now()

		// 프로세스 버퍼에 이벤트를 추가하고 구독자에게 알림
		process.AddEvent(*event)

		// result 이벤트인 경우 데이터를 10분간 메모리에 저장 (방어 로직)
		if event.Type == "result" {
			process.SetResultData(event.Data)
			r.logger.Info().
				Str("processId", process.ID).
				Str("connector", connector.Name()).
				Int("dataSize", len(event.Data)).
				Msg("Result data cached for 10 minutes")
		}

		// CLI 응답 이벤트 로깅
		logEvent := r.logger.Info().
			Str("processId", process.ID).
			Str("connector", connector.Name()).
			Str("eventType", event.Type).
			Time("eventTime", event.Timestamp)

		// 이벤트 데이터 내용 추가 (최대 500자로 제한)
		dataStr := string(event.Data)
		if len(dataStr) > 500 {
			logEvent = logEvent.Str("data", dataStr[:500]+"... (truncated)")
		} else {
			logEvent = logEvent.Str("data", dataStr)
		}

		logEvent.Msg("CLI response event")
	}

	if err := scanner.Err(); err != nil {
		r.logger.Error().
			Str("processId", process.ID).
			Err(err).
			Msg("Error reading output")
	}
}

// handleError는 프로세스 실행 중 오류를 처리합니다
func (r *Runner) handleError(process *Process, err error) {
	r.logger.Error().
		Str("processId", process.ID).
		Err(err).
		Msg("Process error")

	process.SetStatus(StatusFailed)

	// 결과 설정
	result := &Result{
		ExitCode: 1,
		Error:    err.Error(),
	}
	process.SetResult(result)

	// 에러 이벤트 전송
	r.sendErrorEvent(process, err.Error())

	// done 이벤트 전송
	r.sendDoneEvent(process)

	// 프로세스 닫기
	process.Close()
}

// sendErrorEvent는 구독자에게 에러 이벤트를 전송합니다
func (r *Runner) sendErrorEvent(process *Process, errorMsg string) {
	errorData, _ := json.Marshal(map[string]string{
		"error": errorMsg,
	})

	event := Event{
		Type:      "error",
		Data:      errorData,
		Timestamp: time.Now(),
	}

	process.AddEvent(event)
}

// sendDoneEvent는 구독자에게 done 이벤트를 전송합니다
func (r *Runner) sendDoneEvent(process *Process) {
	result := process.GetResult()
	doneData, _ := json.Marshal(map[string]interface{}{
		"processId": process.ID,
		"status":    process.Status,
		"result":    result,
	})

	event := Event{
		Type:      "done",
		Data:      doneData,
		Timestamp: time.Now(),
	}

	process.AddEvent(event)
}

// getExitCode는 명령 에러로부터 종료 코드를 추출합니다
func getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}
