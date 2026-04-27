package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agent-bridge/internal/relay"
)

func run(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runServe(args)
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "status":
		return runStatus(args[1:])
	case "connectors":
		return runConnectors(args[1:])
	case "instances":
		return runInstances(args[1:])
	case "audit":
		return runAudit(args[1:])
	case "enrollment-token":
		return runEnrollmentToken(args[1:])
	case "planner-token":
		return runPlannerToken(args[1:])
	case "connector":
		return runConnectorAdmin(args[1:])
	default:
		return fmt.Errorf("unknown relay command %q", args[0])
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "", "Relay config path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := relay.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	store, err := relay.OpenStore(cfg.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	server := relay.NewServer(cfg, store)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	err = server.Start(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*m = append(*m, value)
	return nil
}

type adminTarget struct {
	relayURL string
	token    string
	asJSON   bool
}

func runStatus(args []string) error {
	target, err := parseAdminTarget("status", args)
	if err != nil {
		return err
	}
	return target.get("/api/v2/status")
}

func runConnectors(args []string) error {
	target, err := parseAdminTarget("connectors", args)
	if err != nil {
		return err
	}
	return target.get("/api/v2/connectors")
}

func runInstances(args []string) error {
	target, err := parseAdminTarget("instances", args)
	if err != nil {
		return err
	}
	return target.get("/api/v2/instances")
}

func runAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	configPath := fs.String("config", "", "Relay config path")
	relayURL := fs.String("relay-url", "", "Relay base URL")
	token := fs.String("token", "", "Planner bearer token")
	plannerName := fs.String("planner-name", "", "Planner token name from config")
	asJSON := fs.Bool("json", false, "Print JSON response")
	limit := fs.Int("limit", 100, "Maximum number of audit events to return")
	if err := fs.Parse(args); err != nil {
		return err
	}

	target, err := resolveAdminTarget(*configPath, *relayURL, *token, *plannerName, *asJSON)
	if err != nil {
		return err
	}
	return target.get(fmt.Sprintf("/api/v2/audit?limit=%d", *limit))
}

func runEnrollmentToken(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: codencer-relayd enrollment-token create [flags]")
	}
	fs := flag.NewFlagSet("enrollment-token create", flag.ContinueOnError)
	configPath := fs.String("config", "", "Relay config path")
	relayURL := fs.String("relay-url", "", "Relay base URL")
	token := fs.String("token", "", "Planner bearer token")
	plannerName := fs.String("planner-name", "", "Planner token name from config")
	label := fs.String("label", "local-dev", "Enrollment token label")
	expiresInSeconds := fs.Int("expires-in-seconds", 600, "Enrollment token lifetime in seconds")
	asJSON := fs.Bool("json", false, "Print JSON response")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	target, err := resolveAdminTarget(*configPath, *relayURL, *token, *plannerName, *asJSON)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"label":              *label,
		"expires_in_seconds": *expiresInSeconds,
	})
	if err != nil {
		return err
	}
	return target.post("/api/v2/connectors/enrollment-tokens", body)
}

func runPlannerToken(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: codencer-relayd planner-token create [flags]")
	}
	fs := flag.NewFlagSet("planner-token create", flag.ContinueOnError)
	configPath := fs.String("config", "", "Relay config path for optional write-back")
	name := fs.String("name", "operator", "Planner token name")
	writeConfig := fs.Bool("write-config", false, "Write the generated token into the relay config file")
	asJSON := fs.Bool("json", false, "Print JSON output")
	entropyBytes := fs.Int("entropy-bytes", 32, "Random entropy bytes before base64url encoding")
	var scopes multiFlag
	var instanceIDs multiFlag
	fs.Var(&scopes, "scope", "Planner scope; repeat to add more")
	fs.Var(&instanceIDs, "instance", "Planner-scoped instance ID; repeat to add more")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *entropyBytes < 16 {
		return fmt.Errorf("entropy-bytes must be at least 16")
	}
	if len(scopes) == 0 {
		scopes = append(scopes, "*")
	}
	token, err := randomBearerToken(*entropyBytes)
	if err != nil {
		return err
	}

	entry := relay.PlannerTokenConfig{
		Name:        *name,
		Token:       token,
		Scopes:      append([]string(nil), scopes...),
		InstanceIDs: append([]string(nil), instanceIDs...),
	}

	if *writeConfig {
		if *configPath == "" {
			return fmt.Errorf("--config is required with --write-config")
		}
		cfg, err := loadRawRelayConfig(*configPath)
		if err != nil {
			return err
		}
		if cfg.PlannerToken != "" {
			cfg.PlannerTokens = append([]relay.PlannerTokenConfig{{
				Name:   "default",
				Token:  cfg.PlannerToken,
				Scopes: []string{"*"},
			}}, cfg.PlannerTokens...)
			cfg.PlannerToken = ""
		}
		cfg.PlannerTokens = upsertPlannerToken(cfg.PlannerTokens, entry)
		if err := relay.SaveConfig(*configPath, cfg); err != nil {
			return err
		}
	}

	output := map[string]any{
		"name":             entry.Name,
		"token":            entry.Token,
		"scopes":           entry.Scopes,
		"instance_ids":     entry.InstanceIDs,
		"config_entry":     entry,
		"config_path":      *configPath,
		"write_config":     *writeConfig,
		"restart_required": true,
	}
	return printOutput(output, *asJSON)
}

func runConnectorAdmin(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: codencer-relayd connector <enable|disable> <connector-id> [flags]")
	}
	action := args[0]
	if action != "enable" && action != "disable" {
		return fmt.Errorf("unknown connector action %q", action)
	}
	connectorID := args[1]

	fs := flag.NewFlagSet("connector "+action, flag.ContinueOnError)
	configPath := fs.String("config", "", "Relay config path")
	relayURL := fs.String("relay-url", "", "Relay base URL")
	token := fs.String("token", "", "Planner bearer token")
	plannerName := fs.String("planner-name", "", "Planner token name from config")
	asJSON := fs.Bool("json", false, "Print JSON response")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}

	target, err := resolveAdminTarget(*configPath, *relayURL, *token, *plannerName, *asJSON)
	if err != nil {
		return err
	}
	return target.post(fmt.Sprintf("/api/v2/connectors/%s/%s", connectorID, action), nil)
}

func parseAdminTarget(command string, args []string) (*adminTarget, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := fs.String("config", "", "Relay config path")
	relayURL := fs.String("relay-url", "", "Relay base URL")
	token := fs.String("token", "", "Planner bearer token")
	plannerName := fs.String("planner-name", "", "Planner token name from config")
	asJSON := fs.Bool("json", false, "Print JSON response")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return resolveAdminTarget(*configPath, *relayURL, *token, *plannerName, *asJSON)
}

func resolveAdminTarget(configPath, relayURL, token, plannerName string, asJSON bool) (*adminTarget, error) {
	target := &adminTarget{relayURL: strings.TrimRight(relayURL, "/"), token: token, asJSON: asJSON}
	if target.relayURL != "" && target.token != "" {
		return target, nil
	}

	cfg, err := relay.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	if target.relayURL == "" {
		target.relayURL = fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	}
	if target.token == "" {
		target.token = plannerTokenFromConfig(cfg, plannerName)
	}
	if target.token == "" {
		return nil, fmt.Errorf("planner bearer token is required; provide --token or configure planner_token(s)")
	}
	return target, nil
}

func (t *adminTarget) get(path string) error {
	req, err := http.NewRequest(http.MethodGet, t.relayURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.do(req)
}

func (t *adminTarget) post(path string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, t.relayURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.token)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	return t.do(req)
}

func (t *adminTarget) do(req *http.Request) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s failed (%d): %s", req.Method, req.URL.String(), resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if t.asJSON {
		fmt.Println(string(body))
		return nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		pretty, err := json.MarshalIndent(payload, "", "  ")
		if err == nil {
			fmt.Println(string(pretty))
			return nil
		}
	}
	fmt.Println(string(body))
	return nil
}

func plannerTokenFromConfig(cfg *relay.Config, name string) string {
	if cfg == nil {
		return ""
	}
	if cfg.PlannerToken != "" {
		return cfg.PlannerToken
	}
	if len(cfg.PlannerTokens) == 0 {
		return ""
	}
	if name == "" {
		return cfg.PlannerTokens[0].Token
	}
	for _, candidate := range cfg.PlannerTokens {
		if candidate.Name == name {
			return candidate.Token
		}
	}
	return ""
}

func randomBearerToken(entropyBytes int) (string, error) {
	buf := make([]byte, entropyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func upsertPlannerToken(values []relay.PlannerTokenConfig, next relay.PlannerTokenConfig) []relay.PlannerTokenConfig {
	for i := range values {
		if values[i].Name == next.Name {
			values[i] = next
			return values
		}
	}
	return append(values, next)
}

func loadRawRelayConfig(path string) (*relay.Config, error) {
	cfg := relay.DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func printOutput(payload any, asJSON bool) error {
	if asJSON {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
