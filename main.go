package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cli-runner/api"
	"cli-runner/config"
	"cli-runner/pkg/logger"

	_ "cli-runner/docs" // Swagger 문서
)

// @title CLI Runner API
// @version 1.0
// @description AI CLI (Claude, Gemini) 실행 및 SSE 스트리밍 서비스
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:4001
// @BasePath /api/v1

// @schemes http https
func main() {
	// 설정 로드
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 로거 설정
	log := logger.Setup(cfg.Logging.Level, cfg.Logging.Format)

	log.Info().Msg("Starting CLI Runner service")
	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Msg("Server configuration loaded")

	// 서버 생성
	server := api.NewServer(cfg, log)
	server.SetupRoutes()

	// 리스너로부터 오는 에러를 수신하는 채널
	serverErrors := make(chan error, 1)

	// 고루틴에서 서버 시작
	go func() {
		serverErrors <- server.Run()
	}()

	// 인터럽트 또는 종료 시그널을 수신하는 채널
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// 시그널 또는 에러를 받을 때까지 블로킹
	select {
	case err := <-serverErrors:
		log.Error().Err(err).Msg("Server error")
		os.Exit(1)

	case sig := <-shutdown:
		log.Info().Str("signal", sig.String()).Msg("Shutdown signal received")
		log.Info().Msg("Server stopped")
	}
}
