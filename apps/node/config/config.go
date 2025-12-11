package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config представляет конфигурацию приложения
type Config struct {
	Node     NodeConfig     `yaml:"node"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	TLS      TLSConfig      `yaml:"tls"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// NodeConfig содержит настройки, специфичные для узла
type NodeConfig struct {
	ID      string `yaml:"id"`
	PartyID string `yaml:"party_id"`
	Port    int    `yaml:"port"`
}

// DatabaseConfig содержит настройки подключения к базе данных
type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	SSLMode         string        `yaml:"ssl_mode"`
	MaxConns        int32         `yaml:"max_conns"`
	MinConns        int32         `yaml:"min_conns"`
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time"`
}

// ConnectionString возвращает URL подключения к PostgreSQL
func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Database, d.SSLMode,
	)
}

// SecurityConfig содержит настройки безопасности
type SecurityConfig struct {
	KeyDir        string `yaml:"key_dir"`
	WhitelistFile string `yaml:"whitelist_file"`
	PeersFile     string `yaml:"peers_file"`
}

// TLSConfig содержит настройки TLS/mTLS
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"`
}

// LoggingConfig содержит настройки логирования
type LoggingConfig struct {
	Level  string `yaml:"level"` // debug, info, warn, error
	Format string `yaml:"format"` // text, json
}

// expandEnvWithDefaults раскрывает переменные окружения с поддержкой значений по умолчанию
// Поддерживает: ${VAR}, ${VAR:default}, $VAR
func expandEnvWithDefaults(s string) string {
	// Pattern for ${VAR:default} or ${VAR}
	re := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

	result := re.ReplaceAllStringFunc(s, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		varName := parts[1]
		defaultVal := ""
		if len(parts) >= 3 {
			defaultVal = parts[2]
		}

		if val := os.Getenv(varName); val != "" {
			return val
		}
		return defaultVal
	})

	// Также раскрывает простой формат $VAR
	return os.ExpandEnv(result)
}

// Load читает конфигурацию из YAML файла
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Раскрытие переменных окружения с поддержкой значений по умолчанию
	expanded := expandEnvWithDefaults(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Установка значений по умолчанию
	cfg.setDefaults()

	return &cfg, nil
}

// setDefaults устанавливает значения по умолчанию для неуказанных полей
func (c *Config) setDefaults() {
	if c.Node.Port == 0 {
		c.Node.Port = 50051
	}

	if c.Database.Port == 0 {
		c.Database.Port = 5432
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "prefer"
	}
	if c.Database.MaxConns == 0 {
		c.Database.MaxConns = 10
	}
	if c.Database.MinConns == 0 {
		c.Database.MinConns = 2
	}
	if c.Database.MaxConnLifetime == 0 {
		c.Database.MaxConnLifetime = time.Hour
	}
	if c.Database.MaxConnIdleTime == 0 {
		c.Database.MaxConnIdleTime = 30 * time.Minute
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
}

// Validate проверяет корректность конфигурации
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database.database is required")
	}

	if c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			return fmt.Errorf("tls.cert_file is required when TLS is enabled")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("tls.key_file is required when TLS is enabled")
		}
		if c.TLS.CAFile == "" {
			return fmt.Errorf("tls.ca_file is required when TLS is enabled")
		}
	}

	return nil
}
