package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"agent-bridge/internal/adapters/claude"
	"agent-bridge/internal/adapters/codex"
	"agent-bridge/internal/adapters/ide"
	"agent-bridge/internal/adapters/qwen"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

// AppContext holds the core dependencies for the daemon.
type AppContext struct {
	Config *Config
	Logger *slog.Logger
	DB     *sql.DB
	Server *http.Server
}

// Bootstrap initializes configuration, logger, storage, artifact paths, and the HTTP server.
func Bootstrap(ctx context.Context, configPath string) (*AppContext, error) {
	// 1. Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// 2. Setup Structured Logger
	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	logger.Info("Starting orchestratord initialization", "version", Version)

	// 3. Initialize SQL database
	db, err := sql.Open("sqlite3", cfg.DBPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}
	// Basic connection check
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite db: %w", err)
	}
	logger.Info("Database connection established", "path", cfg.DBPath)

	// Run migrations
	if err := sqlite.RunMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	logger.Info("Schema migrations complete")

	// 4. Ensure artifact root exists
	if err := os.MkdirAll(cfg.ArtifactRoot, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifact root directory: %w", err)
	}
	logger.Info("Artifact root ready", "path", cfg.ArtifactRoot)

	// 5. Setup basic HTTP handlers (health/version)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"version": Version}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// 6. Application services integration
	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	
	adapters := map[string]domain.Adapter{
		"codex":    codex.NewAdapter(),
		"claude":   claude.NewAdapter(),
		"qwen":     qwen.NewAdapter(),
		"ide-chat": ide.NewAdapter(),
	}

	policyReg := service.NewPolicyRegistry()
	policyDir := filepath.Join(filepath.Dir(cfg.DBPath), "config", "policies")
	if err := policyReg.LoadFromDir(policyDir); err != nil {
		logger.Warn("Failed to load policies from config/policies, using defaults", "error", err)
	}

	routingSvc := service.NewRoutingService(benchmarksRepo, adapters)
	runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo, routingSvc, policyReg, cfg.ArtifactRoot, cfg.WorkspaceRoot)
	gateSvc := service.NewGateService(gatesRepo, runsRepo)

	recoverySvc := service.NewRecoveryService(runsRepo, stepsRepo, attemptsRepo, cfg.ArtifactRoot, cfg.WorkspaceRoot)
	if err := recoverySvc.SweepStaleRuns(ctx); err != nil {
		logger.Warn("Failed to sweep stale runs during bootstrap", "error", err)
	}

	apiHandler := &APIHandler{
		RunSvc:  runSvc,
		GateSvc: gateSvc,
	}
	apiHandler.RegisterRoutes(mux)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	app := &AppContext{
		Config: cfg,
		Logger: logger,
		DB:     db,
		Server: server,
	}

	return app, nil
}

// StartHTTP starts the background web server and blocks until an error or context cancellation occurs.
func (app *AppContext) StartHTTP(ctx context.Context) error {
	app.Logger.Info("Starting HTTP server", "address", app.Server.Addr)
	errChan := make(chan error, 1)
	go func() {
		if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return app.Server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// Close cleans up resources before exit.
func (app *AppContext) Close() error {
	var errs []error
	app.Logger.Info("Shutting down resources")
	if app.DB != nil {
		if err := app.DB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}
	return nil
}
