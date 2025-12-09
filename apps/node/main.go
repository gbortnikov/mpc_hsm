package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/mpc_hsm/node/config"
	"github.com/mpc_hsm/node/db"
	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/security"
	"github.com/mpc_hsm/node/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Флаг конфигурационного файла
	configFile := flag.String("config", "", "Path to YAML config file")

	// CLI флаги (переопределяют конфиг)
	port := flag.Int("port", 0, "gRPC server port (overrides config)")
	nodeID := flag.String("node-id", "", "Node ID (overrides config)")
	partyID := flag.String("party-id", "", "Party ID (overrides config)")
	databaseURL := flag.String("database-url", "", "PostgreSQL connection URL (overrides config)")
	debug := flag.Bool("debug", false, "Enable debug logging (overrides config)")

	flag.Parse()

	// Загружаем конфигурацию
	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Настройка логирования (раннее, чтобы debug логи работали)
	if *debug {
		cfg.Logging.Level = "debug"
	}
	setupLogging(cfg.Logging)

	// CLI флаги переопределяют значения из конфига
	if *port != 0 {
		cfg.Node.Port = *port
	}
	if *nodeID != "" {
		cfg.Node.ID = *nodeID
	}
	if *partyID != "" {
		cfg.Node.PartyID = *partyID
	}
	if *databaseURL != "" {
		slog.Debug("Using database URL from CLI flag")
	}

	slog.Debug("Configuration loaded",
		"config_file", *configFile,
		"node_port", cfg.Node.Port,
		"node_id", cfg.Node.ID,
		"party_id", cfg.Node.PartyID,
		"tls_enabled", cfg.TLS.Enabled,
		"log_level", cfg.Logging.Level,
	)

	// Валидация конфигурации
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	slog.Debug("Configuration validated successfully")

	// Генерируем ID если не указан
	if cfg.Node.ID == "" {
		cfg.Node.ID = fmt.Sprintf("node_%d", cfg.Node.Port)
	}
	if cfg.Node.PartyID == "" {
		cfg.Node.PartyID = fmt.Sprintf("party_%d", cfg.Node.Port)
	}

	// ============================================
	// Инициализация Security Layer
	// ============================================

	var identity *security.NodeIdentity

	if cfg.Security.KeyDir != "" {
		identity, err = security.LoadOrGenerateIdentity(cfg.Node.PartyID, cfg.Security.KeyDir)
		if err != nil {
			log.Fatalf("Failed to load/generate identity: %v", err)
		}
		slog.Info("Identity loaded",
			"party_id", cfg.Node.PartyID,
			"public_key", identity.PublicKeyBase64(),
		)
	} else {
		identity, err = security.GenerateIdentity(cfg.Node.PartyID)
		if err != nil {
			log.Fatalf("Failed to generate identity: %v", err)
		}
		slog.Warn("Using temporary identity (not persisted)")
	}

	// Инициализируем whitelist
	whitelist := security.NewPartyWhitelist()
	if cfg.Security.WhitelistFile != "" {
		if err := whitelist.LoadFromFile(cfg.Security.WhitelistFile); err != nil {
			slog.Warn("Failed to load whitelist", "file", cfg.Security.WhitelistFile, "error", err)
		} else {
			slog.Info("Whitelist loaded", "parties", whitelist.Count())
		}
	}
	whitelist.Add(cfg.Node.PartyID, identity.PublicKey)

	// Инициализируем replay guard
	replayGuard := security.NewReplayGuard(nil)
	defer replayGuard.Stop()

	// Создаём signer и verifier
	signer := security.NewMessageSigner(identity)
	verifier := security.NewMessageVerifier(whitelist, replayGuard, nil)

	// ============================================
	// Настройка TLS
	// ============================================

	var serverOpts []grpc.ServerOption

	if cfg.TLS.Enabled {
		tlsConfig := &security.TLSConfig{
			CertFile: cfg.TLS.CertFile,
			KeyFile:  cfg.TLS.KeyFile,
			CAFile:   cfg.TLS.CAFile,
		}

		if err := security.ValidateTLSConfig(tlsConfig); err != nil {
			log.Fatalf("Invalid TLS config: %v", err)
		}

		creds, err := security.NewServerCredentials(tlsConfig)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials: %v", err)
		}

		serverOpts = append(serverOpts, grpc.Creds(creds))
		slog.Info("mTLS enabled")
	} else {
		slog.Warn("Running without TLS (insecure mode)")
	}

	// ============================================
	// Подключение к базе данных
	// ============================================

	var dbURL string
	if *databaseURL != "" {
		dbURL = *databaseURL
	} else {
		dbURL = cfg.Database.ConnectionString()
	}

	pool, err := db.NewPoolFromURL(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	shareStore := db.NewShareStore(pool)
	slog.Info("Database connected", "host", cfg.Database.Host)

	// ============================================
	// Создание gRPC сервера
	// ============================================

	grpcServer := grpc.NewServer(serverOpts...)

	mpcServer := server.NewMPCNodeServerWithSecurity(
		cfg.Node.ID,
		cfg.Node.PartyID,
		identity,
		signer,
		verifier,
		whitelist,
		replayGuard,
		shareStore,
	)

	pb.RegisterMPCNodeServiceServer(grpcServer, mpcServer)

	// Включаем reflection для отладки
	if cfg.Logging.Level == "debug" {
		reflection.Register(grpcServer)
		slog.Debug("gRPC reflection enabled")
	}

	// ============================================
	// Создание listener
	// ============================================

	addr := fmt.Sprintf(":%d", cfg.Node.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// ============================================
	// P2P Discovery (опционально)
	// ============================================

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Security.PeersFile != "" {
		slog.Debug("P2P peers config loaded", "file", cfg.Security.PeersFile)
	}

	// ============================================
	// Graceful shutdown
	// ============================================

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		slog.Info("Shutting down...")
		cancel()
		grpcServer.GracefulStop()
	}()

	// ============================================
	// Запуск сервера
	// ============================================

	slog.Info("MPC Node started",
		"node_id", cfg.Node.ID,
		"party_id", cfg.Node.PartyID,
		"address", addr,
		"tls", cfg.TLS.Enabled,
	)

	if err := grpcServer.Serve(lis); err != nil && ctx.Err() == nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// loadConfig загружает конфигурацию из файла или использует значения по умолчанию
func loadConfig(configFile string) (*config.Config, error) {
	if configFile != "" {
		return config.Load(configFile)
	}

	// Проверяем переменную окружения
	if envConfig := os.Getenv("MPC_NODE_CONFIG"); envConfig != "" {
		return config.Load(envConfig)
	}

	// Возвращаем конфигурацию по умолчанию
	return &config.Config{
		Node: config.NodeConfig{
			Port: 50051,
		},
		Database: config.DatabaseConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     5432,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Database: os.Getenv("DB_NAME"),
			SSLMode:  "prefer",
			MaxConns: 10,
			MinConns: 2,
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}, nil
}

// setupLogging настраивает логирование на основе конфигурации
func setupLogging(cfg config.LoggingConfig) {
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
