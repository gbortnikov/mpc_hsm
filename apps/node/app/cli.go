package app

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/mpc_hsm/node/config"
)

// CLIFlags содержит все значения флагов командной строки
type CLIFlags struct {
	ConfigFile  string
	Port        int
	NodeID      string
	PartyID     string
	DatabaseURL string
	Debug       bool
}

// ParseFlags разбирает флаги командной строки и возвращает CLIFlags
func ParseFlags() *CLIFlags {
	flags := &CLIFlags{}

	flag.StringVar(&flags.ConfigFile, "config", "", "Path to YAML config file")
	flag.IntVar(&flags.Port, "port", 0, "gRPC server port (overrides config)")
	flag.StringVar(&flags.NodeID, "node-id", "", "Node ID (overrides config)")
	flag.StringVar(&flags.PartyID, "party-id", "", "Party ID (overrides config)")
	flag.StringVar(&flags.DatabaseURL, "database-url", "", "PostgreSQL connection URL (overrides config)")
	flag.BoolVar(&flags.Debug, "debug", false, "Enable debug logging (overrides config)")

	flag.Parse()

	return flags
}

// LoadConfig загружает конфигурацию из файла или окружения и применяет переопределения из CLI
func LoadConfig(flags *CLIFlags) (*config.Config, error) {
	// Загрузка базовой конфигурации
	cfg, err := loadBaseConfig(flags.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Применение переопределений из CLI
	applyFlagOverrides(cfg, flags)

	// Генерация ID по умолчанию, если не указаны
	if cfg.Node.ID == "" {
		cfg.Node.ID = fmt.Sprintf("node_%d", cfg.Node.Port)
	}
	if cfg.Node.PartyID == "" {
		cfg.Node.PartyID = fmt.Sprintf("party_%d", cfg.Node.Port)
	}

	return cfg, nil
}

// loadBaseConfig загружает конфигурацию из файла или возвращает значения по умолчанию
func loadBaseConfig(configFile string) (*config.Config, error) {
	// Попытка загрузки явно указанного файла конфигурации
	if configFile != "" {
		return config.Load(configFile)
	}

	// Попытка загрузки из переменной окружения
	if envConfig := os.Getenv("MPC_NODE_CONFIG"); envConfig != "" {
		return config.Load(envConfig)
	}

	// Возврат конфигурации по умолчанию
	return getDefaultConfig(), nil
}

// getDefaultConfig возвращает конфигурацию по умолчанию с переменными окружения
func getDefaultConfig() *config.Config {
	return &config.Config{
		Node: config.NodeConfig{
			Port: 50051,
		},
		Database: config.DatabaseConfig{
			Host:     getEnvOrDefault("DB_HOST", "localhost"),
			Port:     5432,
			User:     getEnvOrDefault("DB_USER", "postgres"),
			Password: os.Getenv("DB_PASSWORD"),
			Database: getEnvOrDefault("DB_NAME", "mpc_hsm"),
			SSLMode:  "prefer",
			MaxConns: 10,
			MinConns: 2,
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// applyFlagOverrides применяет значения флагов CLI к конфигурации
func applyFlagOverrides(cfg *config.Config, flags *CLIFlags) {
	if flags.Debug {
		cfg.Logging.Level = "debug"
	}

	if flags.Port != 0 {
		cfg.Node.Port = flags.Port
	}

	if flags.NodeID != "" {
		cfg.Node.ID = flags.NodeID
	}

	if flags.PartyID != "" {
		cfg.Node.PartyID = flags.PartyID
	}

	// Примечание: переопределение DatabaseURL обрабатывается отдельно при инициализации базы данных
}

// SetupLogging настраивает глобальный логгер на основе конфигурации
func SetupLogging(cfg config.LoggingConfig) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// LogConfigInfo выводит детали конфигурации для отладки
func LogConfigInfo(cfg *config.Config, configFile string) {
	slog.Debug("Configuration loaded",
		"config_file", configFile,
		"node_port", cfg.Node.Port,
		"node_id", cfg.Node.ID,
		"party_id", cfg.Node.PartyID,
		"tls_enabled", cfg.TLS.Enabled,
		"log_level", cfg.Logging.Level,
	)
}

// getEnvOrDefault возвращает значение переменной окружения или значение по умолчанию
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
