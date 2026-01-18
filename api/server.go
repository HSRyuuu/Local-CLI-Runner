package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"cli-runner/config"
	"cli-runner/connector"
	"cli-runner/runner"
)

// Server는 모든 의존성을 가진 HTTP 서버를 나타냅니다
type Server struct {
	engine   *gin.Engine
	config   *config.Config
	logger   zerolog.Logger
	manager  *runner.Manager
	runner   *runner.Runner
	registry *connector.Registry
	handlers *Handlers
}

// NewServer는 제공된 설정과 로거로 새로운 Server 인스턴스를 생성합니다
func NewServer(cfg *config.Config, logger zerolog.Logger) *Server {
	// 로그 레벨에 따라 Gin 모드 설정 (프로덕션은 release 모드 사용)
	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 매니저 생성
	manager := runner.NewManager(cfg, logger)

	// 러너 생성
	runnerInstance := runner.NewRunner(manager, logger)

	// 커넥터 레지스트리 생성
	registry := connector.NewRegistry()
	registry.SetupFromConfig(cfg)

	// 핸들러 생성
	handlers := NewHandlers(manager, runnerInstance, registry, logger)

	s := &Server{
		engine:   gin.New(),
		config:   cfg,
		logger:   logger,
		manager:  manager,
		runner:   runnerInstance,
		registry: registry,
		handlers: handlers,
	}

	// 미들웨어 추가
	s.engine.Use(gin.Recovery())
	s.engine.Use(s.loggingMiddleware())

	return s
}

// SetupRoutes는 모든 HTTP 라우트를 구성합니다
func (s *Server) SetupRoutes() {
	// 헬스 체크 엔드포인트
	s.engine.GET("/health", s.healthHandler)
	s.engine.GET("/ready", s.readyHandler)

	// Swagger UI
	s.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API 라우트 그룹
	api := s.engine.Group("/api/v1")
	{
		api.POST("/run", s.handlers.RunHandler)
		api.GET("/stream/:id", s.handlers.StreamHandler)
		api.GET("/process/:id", s.handlers.GetProcessHandler)
		api.GET("/result/:id", s.handlers.GetResultHandler)
		api.GET("/result-data/:id", s.handlers.GetResultDataHandler)
		api.DELETE("/process/:id", s.handlers.DeleteProcessHandler)
		api.GET("/processes", s.handlers.ListProcessesHandler)
		api.GET("/connectors", s.handlers.ListConnectorsHandler)
	}

	// 클린업 고루틴 시작
	s.manager.StartCleanup()
}

// Run은 HTTP 서버를 시작합니다
func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	s.logger.Info().
		Str("address", addr).
		Msg("Starting HTTP server")

	return s.engine.Run(addr)
}

// healthHandler는 기본 상태 정보를 반환합니다
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// readyHandler는 서버가 요청을 받을 준비가 되었는지 확인합니다
func (s *Server) readyHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// loggingMiddleware는 zerolog을 사용하는 요청 로깅을 위한 Gin 미들웨어를 생성합니다
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// SSE 스트림에 대한 로깅 건너뛰기 (별도로 로깅됨)
		if c.Request.URL.Path != "/api/v1/stream" {
			// 요청 처리
			c.Next()

			// 요청 처리 후 로깅
			s.logger.Info().
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Int("status", c.Writer.Status()).
				Str("ip", c.ClientIP()).
				Msg("HTTP request")
		} else {
			c.Next()
		}
	}
}
