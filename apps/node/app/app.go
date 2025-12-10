package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mpc_hsm/node/config"
	"github.com/mpc_hsm/node/db"
	"github.com/mpc_hsm/node/middleware"
	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/security"
	"github.com/mpc_hsm/node/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Application manages the lifecycle of the MPC Node application
type Application struct {
	config      *config.Config
	ctx         context.Context
	cancel      context.CancelFunc
	grpcServer  *grpc.Server
	listener    net.Listener
	components  *Components
	initialized bool
}

// Components holds all initialized application components
type Components struct {
	Config       *config.Config
	Identity     *security.NodeIdentity
	Signer       *security.MessageSigner
	Verifier     *security.MessageVerifier
	Whitelist    *security.PartyWhitelist
	ReplayGuard  *security.ReplayGuard
	DBPool       *pgxpool.Pool
	ShareStore   *db.ShareStore
	MPCServer    *server.MPCNodeServer
	GRPCServer   *grpc.Server
	Listener     net.Listener
}

// New creates a new Application instance
func New(cfg *config.Config) *Application {
	ctx, cancel := context.WithCancel(context.Background())
	return &Application{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Initialize sets up all application components
func (a *Application) Initialize() error {
	slog.Info("Initializing application components...")

	// Validate configuration
	if err := a.config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize components in order
	components := &Components{
		Config: a.config,
	}

	// Security layer
	if err := a.initSecurity(components); err != nil {
		return fmt.Errorf("failed to initialize security: %w", err)
	}

	// Database
	if err := a.initDatabase(components); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// gRPC server
	if err := a.initGRPCServer(components); err != nil {
		return fmt.Errorf("failed to initialize gRPC server: %w", err)
	}

	a.components = components
	a.initialized = true

	slog.Info("Application components initialized successfully")
	return nil
}

// initSecurity initializes security components
func (a *Application) initSecurity(c *Components) error {
	slog.Debug("Initializing security components...")

	// Generate or load identity
	var identity *security.NodeIdentity
	var err error

	if a.config.Security.KeyDir != "" {
		identity, err = security.LoadOrGenerateIdentity(a.config.Node.PartyID, a.config.Security.KeyDir)
		if err != nil {
			return fmt.Errorf("failed to load/generate identity: %w", err)
		}
		slog.Info("Identity loaded",
			"party_id", a.config.Node.PartyID,
			"public_key", identity.PublicKeyBase64(),
		)
	} else {
		identity, err = security.GenerateIdentity(a.config.Node.PartyID)
		if err != nil {
			return fmt.Errorf("failed to generate identity: %w", err)
		}
		slog.Warn("Using temporary identity (not persisted)")
	}
	c.Identity = identity

	// Initialize whitelist
	whitelist := security.NewPartyWhitelist()
	if a.config.Security.WhitelistFile != "" {
		if err := whitelist.LoadFromFile(a.config.Security.WhitelistFile); err != nil {
			slog.Warn("Failed to load whitelist",
				"file", a.config.Security.WhitelistFile,
				"error", err,
			)
		} else {
			slog.Info("Whitelist loaded", "parties", whitelist.Count())
		}
	}
	whitelist.Add(a.config.Node.PartyID, identity.PublicKey)
	c.Whitelist = whitelist

	// Initialize replay guard
	replayGuard := security.NewReplayGuard(nil)
	c.ReplayGuard = replayGuard

	// Create signer and verifier
	c.Signer = security.NewMessageSigner(identity)
	c.Verifier = security.NewMessageVerifier(whitelist, replayGuard, nil)

	slog.Debug("Security components initialized")
	return nil
}

// initDatabase initializes database connection and stores
func (a *Application) initDatabase(c *Components) error {
	slog.Debug("Initializing database connection...")

	dbURL := a.config.Database.ConnectionString()
	pool, err := db.NewPoolFromURL(a.ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	c.DBPool = pool

	c.ShareStore = db.NewShareStore(pool)

	slog.Info("Database connected", "host", a.config.Database.Host)
	return nil
}

// initGRPCServer initializes the gRPC server and MPC service
func (a *Application) initGRPCServer(c *Components) error {
	slog.Debug("Initializing gRPC server...")

	// Prepare gRPC server options
	var serverOpts []grpc.ServerOption

	// Add middleware interceptors
	serverOpts = append(serverOpts,
		grpc.ChainUnaryInterceptor(
			middleware.RecoveryUnaryInterceptor(),
			middleware.LoggingUnaryInterceptor(),
			middleware.AuthUnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			middleware.RecoveryStreamInterceptor(),
			middleware.LoggingStreamInterceptor(),
			middleware.AuthStreamInterceptor(),
		),
	)

	// Configure TLS if enabled
	if a.config.TLS.Enabled {
		tlsConfig := &security.TLSConfig{
			CertFile: a.config.TLS.CertFile,
			KeyFile:  a.config.TLS.KeyFile,
			CAFile:   a.config.TLS.CAFile,
		}

		if err := security.ValidateTLSConfig(tlsConfig); err != nil {
			return fmt.Errorf("invalid TLS config: %w", err)
		}

		creds, err := security.NewServerCredentials(tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to create TLS credentials: %w", err)
		}

		serverOpts = append(serverOpts, grpc.Creds(creds))
		slog.Info("mTLS enabled")
	} else {
		slog.Warn("Running without TLS (insecure mode)")
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(serverOpts...)
	c.GRPCServer = grpcServer

	// Create MPC server
	mpcServer := server.NewMPCNodeServerWithSecurity(
		a.config.Node.ID,
		a.config.Node.PartyID,
		c.Identity,
		c.Signer,
		c.Verifier,
		c.Whitelist,
		c.ReplayGuard,
		c.ShareStore,
	)
	c.MPCServer = mpcServer

	// Register service
	pb.RegisterMPCNodeServiceServer(grpcServer, mpcServer)

	// Enable reflection for debugging
	if a.config.Logging.Level == "debug" {
		reflection.Register(grpcServer)
		slog.Debug("gRPC reflection enabled")
	}

	// Create listener
	addr := fmt.Sprintf(":%d", a.config.Node.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	c.Listener = listener

	slog.Debug("gRPC server initialized", "address", addr)
	return nil
}

// Run starts the application and blocks until shutdown
func (a *Application) Run() error {
	if !a.initialized {
		return fmt.Errorf("application not initialized, call Initialize() first")
	}

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start gRPC server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		slog.Info("MPC Node started",
			"node_id", a.config.Node.ID,
			"party_id", a.config.Node.PartyID,
			"address", a.components.Listener.Addr().String(),
			"tls", a.config.TLS.Enabled,
		)

		if err := a.components.GRPCServer.Serve(a.components.Listener); err != nil {
			errCh <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		slog.Info("Received shutdown signal", "signal", sig)
		return a.Shutdown()
	case err := <-errCh:
		slog.Error("Server error", "error", err)
		return err
	}
}

// Shutdown gracefully shuts down the application
func (a *Application) Shutdown() error {
	slog.Info("Shutting down application...")

	// Cancel context
	a.cancel()

	// Gracefully stop gRPC server
	if a.components != nil && a.components.GRPCServer != nil {
		slog.Debug("Stopping gRPC server...")
		a.components.GRPCServer.GracefulStop()
	}

	// Stop replay guard
	if a.components != nil && a.components.ReplayGuard != nil {
		slog.Debug("Stopping replay guard...")
		a.components.ReplayGuard.Stop()
	}

	// Close database connection
	if a.components != nil && a.components.DBPool != nil {
		slog.Debug("Closing database connection...")
		a.components.DBPool.Close()
	}

	slog.Info("Application shutdown complete")
	return nil
}

// GetComponents returns the initialized components
func (a *Application) GetComponents() *Components {
	return a.components
}

// Context returns the application context
func (a *Application) Context() context.Context {
	return a.ctx
}
