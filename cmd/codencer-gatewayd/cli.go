package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"agent-bridge/internal/gateway"
	"agent-bridge/internal/local"
)

func run(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runServe(args)
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:])
	default:
		return fmt.Errorf("unknown gateway command %q", args[0])
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "", "Gateway config path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := strings.TrimSpace(*configPath)
	if path == "" {
		paths, err := local.ResolvePaths("", "")
		if err != nil {
			return err
		}
		path = filepath.Join(paths.RuntimeDir, "gateway", "config.json")
	}
	cfg, err := gateway.LoadConfig(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Store.Path) == "" && len(cfg.RelayProfiles) == 0 {
		cfg.Store.Path = filepath.Join(filepath.Dir(path), "gateway.db")
	}
	server, err := gateway.NewServer(cfg, gateway.ServerOptions{})
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	err = server.Start(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
