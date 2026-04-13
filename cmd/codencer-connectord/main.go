package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agent-bridge/internal/connector"
)

const defaultConnectorConfigPath = ".codencer/connector/config.json"

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: codencer-connectord <enroll|run|status|list|share|unshare|config> [flags]")
	}

	switch args[0] {
	case "enroll":
		return runEnroll(ctx, args[1:], stdout, stderr)
	case "run":
		return runConnector(ctx, args[1:], stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "share":
		return runShare(ctx, args[1:], stdout, stderr)
	case "unshare":
		return runUnshare(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown connector command %s", args[0])
	}
}

func runEnroll(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("enroll", stderr)
	relayURL := fs.String("relay-url", "http://127.0.0.1:8090", "Relay base URL")
	daemonURL := fs.String("daemon-url", "http://127.0.0.1:8085", "Local Codencer daemon URL")
	enrollmentToken := fs.String("enrollment-token", "", "Relay enrollment token")
	configPath := fs.String("config", defaultConnectorConfigPath, "Connector config path")
	label := fs.String("label", "", "Optional connector label")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := connector.Enroll(ctx, *relayURL, *daemonURL, *enrollmentToken, *label, *configPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Connector enrolled: %s machine=%s\n", cfg.ConnectorID, cfg.MachineID)
	return err
}

func runConnector(ctx context.Context, args []string, stderr io.Writer) error {
	fs := newFlagSet("run", stderr)
	configPath := fs.String("config", defaultConnectorConfigPath, "Connector config path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := connector.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	client := connector.NewClient(cfg)

	runCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := client.Run(runCtx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func runStatus(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("status", stderr)
	configPath := fs.String("config", defaultConnectorConfigPath, "Connector config path")
	jsonOutput := fs.Bool("json", false, "Print raw connector status JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	status, err := connector.LoadStatus(*configPath)
	if err != nil {
		return err
	}

	if *jsonOutput {
		data, err := os.ReadFile(connector.StatusPathForConfig(*configPath))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}

	cfg, err := connector.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	entries := connector.EffectiveSharedInstances(cfg)

	if _, err := fmt.Fprintf(stdout, "connector=%s machine=%s relay=%s state=%s\n",
		status.ConnectorID,
		status.MachineID,
		status.RelayURL,
		status.SessionState,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "last_connect=%s last_disconnect=%s last_heartbeat=%s\n",
		blankOrValue(status.LastConnectAt),
		blankOrValue(status.LastDisconnectAt),
		blankOrValue(status.LastHeartbeatAt),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "shared_now=%s\n", formatList(status.SharedInstances)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "configured_instances=%d shared_config=%d unshared_config=%d\n",
		len(entries),
		countInstances(entries, true),
		countInstances(entries, false),
	); err != nil {
		return err
	}
	if status.LastError != "" {
		if _, err := fmt.Fprintf(stdout, "last_error=%s\n", status.LastError); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		if _, err := fmt.Fprintf(stdout, "%s\n", formatInstanceLine(entry)); err != nil {
			return err
		}
	}
	return nil
}

func runList(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("list", stderr)
	configPath := fs.String("config", defaultConnectorConfigPath, "Connector config path")
	jsonOutput := fs.Bool("json", false, "Print shared instance config as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := connector.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	entries := connector.EffectiveSharedInstances(cfg)
	if *jsonOutput {
		return writeJSON(stdout, entries)
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(stdout, "no configured connector instances")
		return err
	}
	for _, entry := range entries {
		if _, err := fmt.Fprintln(stdout, formatInstanceLine(entry)); err != nil {
			return err
		}
	}
	return nil
}

func runShare(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("share", stderr)
	configPath := fs.String("config", defaultConnectorConfigPath, "Connector config path")
	instanceID := fs.String("instance-id", "", "Instance ID to share")
	daemonURL := fs.String("daemon-url", "", "Daemon URL to share")
	manifestPath := fs.String("manifest-path", "", "Manifest path to share")
	jsonOutput := fs.Bool("json", false, "Print the updated entry as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := connector.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	entry, err := connector.ShareInstance(ctx, cfg, connector.InstanceSelector{
		InstanceID:   *instanceID,
		DaemonURL:    *daemonURL,
		ManifestPath: *manifestPath,
	}, nil)
	if err != nil {
		return err
	}
	if err := connector.SaveConfig(*configPath, cfg); err != nil {
		return err
	}
	if err := connector.NewStatusStore(*configPath).SyncConfig(cfg); err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, entry)
	}
	_, err = fmt.Fprintf(stdout, "shared %s\n", formatInstanceLine(entry))
	return err
}

func runUnshare(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("unshare", stderr)
	configPath := fs.String("config", defaultConnectorConfigPath, "Connector config path")
	instanceID := fs.String("instance-id", "", "Instance ID to unshare")
	daemonURL := fs.String("daemon-url", "", "Daemon URL to unshare")
	manifestPath := fs.String("manifest-path", "", "Manifest path to unshare")
	jsonOutput := fs.Bool("json", false, "Print the updated entry as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := connector.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	entry, err := connector.UnshareInstance(cfg, connector.InstanceSelector{
		InstanceID:   *instanceID,
		DaemonURL:    *daemonURL,
		ManifestPath: *manifestPath,
	})
	if err != nil {
		return err
	}
	if err := connector.SaveConfig(*configPath, cfg); err != nil {
		return err
	}
	if err := connector.NewStatusStore(*configPath).SyncConfig(cfg); err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, entry)
	}
	_, err = fmt.Fprintf(stdout, "unshared %s\n", formatInstanceLine(entry))
	return err
}

func runConfig(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("config", stderr)
	configPath := fs.String("config", defaultConnectorConfigPath, "Connector config path")
	jsonOutput := fs.Bool("json", false, "Print connector config as JSON")
	showSecrets := fs.Bool("show-secrets", false, "Include sensitive config values in output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := connector.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if *jsonOutput {
		data, err := connector.MarshalConfig(cfg, *showSecrets)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}

	safeCfg := connector.RedactedConfig(cfg, *showSecrets)
	if _, err := fmt.Fprintf(stdout, "relay=%s websocket=%s connector=%s machine=%s label=%s heartbeat_seconds=%d\n",
		safeCfg.RelayURL,
		blankOrValue(safeCfg.WebsocketURL),
		blankOrValue(safeCfg.ConnectorID),
		blankOrValue(safeCfg.MachineID),
		blankOrValue(safeCfg.Label),
		safeCfg.HeartbeatIntervalSeconds,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "private_key=%s public_key=%s\n",
		blankOrValue(safeCfg.PrivateKey),
		blankOrValue(safeCfg.PublicKey),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "discovery_roots=%s\n", formatList(safeCfg.DiscoveryRoots)); err != nil {
		return err
	}
	for _, entry := range connector.EffectiveSharedInstances(safeCfg) {
		if _, err := fmt.Fprintln(stdout, formatInstanceLine(entry)); err != nil {
			return err
		}
	}
	return nil
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func countInstances(entries []connector.SharedInstanceConfig, share bool) int {
	count := 0
	for _, entry := range entries {
		if entry.Share == share {
			count++
		}
	}
	return count
}

func blankOrValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func formatList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func formatInstanceLine(entry connector.SharedInstanceConfig) string {
	state := "unshared"
	if entry.Share {
		state = "shared"
	}
	return fmt.Sprintf("state=%s instance_id=%s daemon_url=%s manifest_path=%s",
		state,
		blankOrValue(entry.InstanceID),
		blankOrValue(entry.DaemonURL),
		blankOrValue(entry.ManifestPath),
	)
}

func writeJSON(stdout io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, string(data))
	return err
}
