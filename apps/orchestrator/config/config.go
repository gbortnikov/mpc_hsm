package config

import (
	"os"
	"strconv"
	"time"
)

// Config содержит настройки оркестратора
type Config struct {
	Server  ServerConfig
	Session SessionConfig
	TLS     TLSConfig
}

// ServerConfig содержит настройки HTTP сервера
type ServerConfig struct {
	Port                  string
	EnablePlayground      bool
	EnableIntrospection   bool
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	MaxQueryCacheSize     int
	MaxAPQCacheSize       int
	WebSocketPingInterval time.Duration
}

// SessionConfig содержит настройки для MPC сессий
type SessionConfig struct {
	MaxIterations         int
	IterationSleepMs      int
	InitializationSleepMs int
	NoMessageTimeout      int
	RequestTimeoutSec     int
}

// TLSConfig содержит настройки TLS для gRPC соединений
type TLSConfig struct {
	Enabled    bool
	CertFile   string
	KeyFile    string
	CAFile     string
	ServerName string
}

// Load загружает конфигурацию из переменных окружения или использует значения по умолчанию
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:                  getEnv("PORT", "8081"),
			EnablePlayground:      getEnvBool("ENABLE_PLAYGROUND", true),
			EnableIntrospection:   getEnvBool("ENABLE_INTROSPECTION", true),
			ReadTimeout:           getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:          getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			MaxQueryCacheSize:     getEnvInt("MAX_QUERY_CACHE_SIZE", 1000),
			MaxAPQCacheSize:       getEnvInt("MAX_APQ_CACHE_SIZE", 100),
			WebSocketPingInterval: getEnvDuration("WEBSOCKET_PING_INTERVAL", 10*time.Second),
		},
		Session: SessionConfig{
			MaxIterations:         getEnvInt("SESSION_MAX_ITERATIONS", 100),
			IterationSleepMs:      getEnvInt("SESSION_ITERATION_SLEEP_MS", 100),
			InitializationSleepMs: getEnvInt("SESSION_INIT_SLEEP_MS", 1000),
			NoMessageTimeout:      getEnvInt("SESSION_NO_MESSAGE_TIMEOUT", 30),
			RequestTimeoutSec:     getEnvInt("SESSION_REQUEST_TIMEOUT_SEC", 10),
		},
		TLS: TLSConfig{
			Enabled:    getEnvBool("TLS_ENABLED", false),
			CertFile:   getEnv("TLS_CERT_FILE", "certs/client.crt"),
			KeyFile:    getEnv("TLS_KEY_FILE", "certs/client.key"),
			CAFile:     getEnv("TLS_CA_FILE", "certs/ca.crt"),
			ServerName: getEnv("TLS_SERVER_NAME", ""),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
	}
	return defaultValue
}
