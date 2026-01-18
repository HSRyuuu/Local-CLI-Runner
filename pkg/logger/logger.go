package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Setup은 제공된 레벨과 형식을 기반으로 구성된 zerolog.Logger를 초기화하고 반환합니다.
// Level은 다음 중 하나여야 합니다: "debug", "info", "warn", "error"
// Format은 다음 중 하나여야 합니다: "json", "console"
func Setup(level, format string) zerolog.Logger {
	// 시간 형식 구성
	zerolog.TimeFieldFormat = time.RFC3339

	// 로그 레벨 파싱 및 설정
	logLevel := parseLevel(level)
	zerolog.SetGlobalLevel(logLevel)

	// 출력 형식 구성
	var logger zerolog.Logger
	if strings.ToLower(format) == "console" {
		logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}).With().Timestamp().Logger()
	} else {
		// 기본값은 JSON 형식
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	return logger
}

// parseLevel은 문자열 로그 레벨을 zerolog.Level로 변환합니다
func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel // 알 수 없는 경우 기본값은 info
	}
}

// 일반적인 로깅 패턴을 위한 헬퍼 함수

// WithError는 로그 컨텍스트에 에러 필드를 추가합니다
func WithError(logger zerolog.Logger, err error) *zerolog.Event {
	return logger.Error().Err(err)
}

// WithFields는 로그 컨텍스트에 여러 필드를 추가합니다
func WithFields(logger zerolog.Logger, fields map[string]interface{}) zerolog.Logger {
	ctx := logger.With()
	for key, value := range fields {
		ctx = ctx.Interface(key, value)
	}
	return ctx.Logger()
}

// LogRequest는 HTTP 요청을 로깅하기 위한 헬퍼입니다
func LogRequest(logger zerolog.Logger, method, path string, statusCode int, duration time.Duration) {
	logger.Info().
		Str("method", method).
		Str("path", path).
		Int("status", statusCode).
		Dur("duration", duration).
		Msg("HTTP request")
}

// LogCommand는 명령 실행을 로깅하기 위한 헬퍼입니다
func LogCommand(logger zerolog.Logger, command string, args []string, exitCode int, duration time.Duration) {
	logger.Info().
		Str("command", command).
		Strs("args", args).
		Int("exit_code", exitCode).
		Dur("duration", duration).
		Msg("Command executed")
}

// LogCommandError는 명령 실행 오류를 로깅하기 위한 헬퍼입니다
func LogCommandError(logger zerolog.Logger, command string, args []string, err error) {
	logger.Error().
		Err(err).
		Str("command", command).
		Strs("args", args).
		Msg("Command execution failed")
}

// LogConnectorEvent는 커녅터 관련 이벤트를 로깅하기 위한 헬퍼입니다
func LogConnectorEvent(logger zerolog.Logger, connectorType, event string, details map[string]interface{}) {
	evt := logger.Info().
		Str("connector_type", connectorType).
		Str("event", event)

	for key, value := range details {
		evt = evt.Interface(key, value)
	}

	evt.Msg("Connector event")
}
