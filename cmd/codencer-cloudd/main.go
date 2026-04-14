package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"agent-bridge/internal/cloud"
	cloudconnectors "agent-bridge/internal/cloud/connectors"
	"agent-bridge/internal/relay"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("codencer-cloudd", flag.ContinueOnError)
	configPath := fs.String("config", "", "Cloud config path")
	relayConfigPath := fs.String("relay-config", "", "Relay config path to compose under the cloud service")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := cloud.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if *relayConfigPath == "" {
		*relayConfigPath = cfg.RelayConfigPath
	}

	store, err := cloud.OpenStore(cfg.DBPath, cfg.MasterKey)
	if err != nil {
		return err
	}
	defer store.Close()

	var relayHandler http.Handler
	var relayStore *relay.Store
	if *relayConfigPath != "" {
		relayCfg, err := relay.LoadConfig(*relayConfigPath)
		if err != nil {
			return err
		}
		relayStore, err = relay.OpenStore(relayCfg.DBPath)
		if err != nil {
			return err
		}
		defer relayStore.Close()
		relayHandler = relay.NewServer(relayCfg, relayStore).Handler()
	}

	server := cloud.NewServer(cfg, store, cloudconnectors.NewRegistry(), relayHandler)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	return server.Start(ctx)
}
