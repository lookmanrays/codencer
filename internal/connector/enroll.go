package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"agent-bridge/internal/relayproto"
)

func Enroll(ctx context.Context, relayURL, daemonURL, enrollmentToken, label, configPath string) (*Config, error) {
	cfg := &Config{
		RelayURL:                 strings.TrimRight(relayURL, "/"),
		DaemonURL:                strings.TrimRight(daemonURL, "/"),
		Label:                    label,
		HeartbeatIntervalSeconds: DefaultHeartbeatIntervalSeconds,
	}
	if err := EnsureKeypair(cfg); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(relayproto.EnrollmentRequest{
		EnrollmentToken:  enrollmentToken,
		EnrollmentSecret: enrollmentToken,
		Label:            label,
		PublicKey:        cfg.PublicKey,
		Machine:          CurrentMachineMetadata(),
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.RelayURL+"/api/v2/connectors/enroll", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("connector enrollment failed: %s", string(body))
	}

	var enrollment relayproto.EnrollmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&enrollment); err != nil {
		return nil, err
	}

	cfg.ConnectorID = enrollment.ConnectorID
	cfg.MachineID = enrollment.MachineID
	if enrollment.Relay.RelayURL != "" {
		cfg.RelayURL = strings.TrimRight(enrollment.Relay.RelayURL, "/")
	}
	cfg.WebsocketURL = strings.TrimRight(enrollment.Relay.WebsocketURL, "/")
	if enrollment.Relay.HeartbeatIntervalSeconds > 0 {
		cfg.HeartbeatIntervalSeconds = enrollment.Relay.HeartbeatIntervalSeconds
	}

	if daemonURL != "" {
		instanceClient := NewCodencerClient(cfg.DaemonURL)
		if info, err := instanceClient.GetInstance(ctx); err == nil {
			cfg.UpsertSharedInstance(SharedInstanceConfig{
				InstanceID:   info.ID,
				DaemonURL:    cfg.DaemonURL,
				ManifestPath: info.ManifestPath,
				Share:        true,
			})
		} else {
			cfg.UpsertSharedInstance(SharedInstanceConfig{
				DaemonURL: cfg.DaemonURL,
				Share:     true,
			})
		}
	}

	if err := SaveConfig(configPath, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
