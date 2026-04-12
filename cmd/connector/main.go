package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"agent-bridge/internal/connector"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: connector <enroll|run> [flags]")
	}

	switch os.Args[1] {
	case "enroll":
		runEnroll(os.Args[2:])
	case "run":
		runConnector(os.Args[2:])
	default:
		log.Fatalf("unknown connector command %s", os.Args[1])
	}
}

func runEnroll(args []string) {
	fs := flag.NewFlagSet("enroll", flag.ExitOnError)
	relayURL := fs.String("relay-url", "http://127.0.0.1:8090", "Relay base URL")
	daemonURL := fs.String("daemon-url", "http://127.0.0.1:8085", "Local Codencer daemon URL")
	enrollmentSecret := fs.String("enrollment-secret", "", "Relay enrollment token (legacy flag name)")
	enrollmentToken := fs.String("enrollment-token", "", "Relay enrollment token")
	configPath := fs.String("config", ".codencer/connector/config.json", "Connector config path")
	label := fs.String("label", "", "Optional connector label")
	fs.Parse(args)

	token := *enrollmentToken
	if token == "" {
		token = *enrollmentSecret
	}
	cfg, err := connector.Enroll(context.Background(), *relayURL, *daemonURL, token, *label, *configPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Connector enrolled: %s machine=%s\n", cfg.ConnectorID, cfg.MachineID)
}

func runConnector(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", ".codencer/connector/config.json", "Connector config path")
	fs.Parse(args)

	cfg, err := connector.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	client := connector.NewClient(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := client.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
