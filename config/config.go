package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config는 애플리케이션 설정을 나타냅니다
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Process    ProcessConfig    `mapstructure:"process"`
	Connectors ConnectorsConfig `mapstructure:"connectors"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig는 HTTP 서버 설정을 포함합니다
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Host         string        `mapstructure:"host"`
	ReadTimeout  time.Duration `mapstructure:"readTimeout"`
	WriteTimeout time.Duration `mapstructure:"writeTimeout"`
}

// ProcessConfig는 프로세스 실행 설정을 포함합니다
type ProcessConfig struct {
	DefaultTimeout time.Duration `mapstructure:"defaultTimeout"`
	MaxConcurrent  int           `mapstructure:"maxConcurrent"`
	CleanupDelay   time.Duration `mapstructure:"cleanupDelay"`
	BufferSize     int           `mapstructure:"bufferSize"`
}

// ConnectorConfig는 단일 커넥터의 설정을 포함합니다
type ConnectorConfig struct {
	Command   string   `mapstructure:"command"`
	Args      []string `mapstructure:"args"`
	Available bool     `mapstructure:"available"`
}

// ConnectorsConfig는 모든 커녅터 설정을 포함합니다
type ConnectorsConfig struct {
	Claude ConnectorConfig `mapstructure:"claude"`
}

// LoggingConfig는 로깅 설정을 포함합니다
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load는 config.yaml과 환경 변수로부터 설정을 읽습니다
// 환경 변수는 CLI_RUNNER_ 접두사가 붙으며 파일 값을 재정의합니다
func Load() (*Config, error) {
	v := viper.New()

	// 설정 파일 설정
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// 환경 변수 재정의 활성화
	v.SetEnvPrefix("CLI_RUNNER")
	v.AutomaticEnv()

	// 기본값 설정
	setDefaults(v)

	// 설정 파일 읽기 (선택사항 - 찾을 수 없으면 기본값 사용)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// 설정 파일을 찾을 수 없음; 기본값 및 환경 변수 사용
	}

	// 설정을 구조체로 언마셜링
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults는 합리적인 기본값을 구성합니다
func setDefaults(v *viper.Viper) {
	// 서버 기본값
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.readTimeout", 30*time.Second)
	v.SetDefault("server.writeTimeout", 30*time.Second)

	// 프로세스 기본값
	v.SetDefault("process.defaultTimeout", 5*time.Minute)
	v.SetDefault("process.maxConcurrent", 10)
	v.SetDefault("process.cleanupDelay", 5*time.Second)
	v.SetDefault("process.bufferSize", 8192)

	// 커녅터 기본값 - Claude
	v.SetDefault("connectors.claude.command", "claude")
	v.SetDefault("connectors.claude.args", []string{})
	v.SetDefault("connectors.claude.available", true)

	// 로깅 기본값
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}
