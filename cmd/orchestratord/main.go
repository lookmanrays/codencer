package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"agent-bridge/internal/app"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env") // Load env config before bootstrapping
	
	configPath := flag.String("config", "", "Path to configuration file")
	repoRoot := flag.String("repo-root", "", "Explicit path to the repository root (overrides CWD)")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initializing signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	appCtx, err := app.Bootstrap(ctx, *configPath, *repoRoot)
	if err != nil {
		slog.Error("Failed to bootstrap application", "error", err)
		os.Exit(1)
	}
	defer appCtx.Close()

	// Error channel for the server
	serverErr := make(chan error, 1)
	go func() {
		if err := appCtx.StartHTTP(ctx); err != nil {
			serverErr <- err
		}
	}()

	// Wait for termination signal or server error
	select {
	case sig := <-sigChan:
		appCtx.Logger.Info("Received termination signal", "signal", sig)
		cancel()
	case err := <-serverErr:
		appCtx.Logger.Error("Server error", "error", err)
		cancel()
		os.Exit(1)
	}

	appCtx.Logger.Info("Daemon stopped gracefully")
}
