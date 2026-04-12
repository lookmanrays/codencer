package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"agent-bridge/internal/relay"
)

func main() {
	configPath := flag.String("config", "", "Relay config path")
	flag.Parse()

	cfg, err := relay.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	store, err := relay.OpenStore(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(cfg, store)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := server.Start(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
