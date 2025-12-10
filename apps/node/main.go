package main

import (
	"log"
	"os"

	"github.com/mpc_hsm/node/app"
)

func main() {
	// Parse command-line flags
	flags := app.ParseFlags()

	// Load configuration
	cfg, err := app.LoadConfig(flags)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup logging early so debug logs work
	app.SetupLogging(cfg.Logging)
	app.LogConfigInfo(cfg, flags.ConfigFile)

	// Create and initialize application
	application := app.New(cfg)
	if err := application.Initialize(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Run application and handle shutdown
	if err := application.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
		os.Exit(1)
	}
}
