package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agent-bridge/internal/cloud"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("codencer-cloudworkerd", flag.ContinueOnError)
	configPath := fs.String("config", "", "Cloud config path")
	interval := fs.Duration("interval", 2*time.Minute, "Polling interval for cloud worker jobs")
	pollLimit := fs.Int("limit", 50, "Maximum provider records to poll per installation pass")
	once := fs.Bool("once", false, "Run one worker pass and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := cloud.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	store, err := cloud.OpenStore(cfg.DBPath, cfg.MasterKey)
	if err != nil {
		return err
	}
	defer store.Close()

	worker := cloud.NewWorker(store, nil, *pollLimit)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *once {
		return worker.RunOnce(ctx)
	}
	return worker.Run(ctx, *interval)
}
