package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"agent-bridge/internal/relayproto"
	"github.com/gorilla/websocket"
)

type Client struct {
	cfgMu      sync.RWMutex
	cfg        *Config
	httpClient *http.Client
	status     *StatusStore
	backoff    *Backoff
	dialer     *websocket.Dialer
}

func NewClient(cfg *Config) *Client {
	return &Client{
		cfg:        cfg.Clone(),
		httpClient: &http.Client{Timeout: 15 * time.Second},
		status:     NewStatusStore(cfg.ConfigPath),
		backoff:    NewBackoff(500*time.Millisecond, 10*time.Second),
		dialer:     websocket.DefaultDialer,
	}
}

func (c *Client) Run(ctx context.Context) error {
	for {
		err := c.runOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			c.backoff.Reset()
			continue
		}
		delay := c.backoff.Next()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Client) runOnce(ctx context.Context) (err error) {
	cfg := c.currentConfig()
	if c.status != nil {
		_ = c.status.MarkConnecting(cfg)
	}
	defer func() {
		if c.status == nil {
			return
		}
		now := time.Now().UTC()
		switch {
		case ctx.Err() != nil:
			_ = c.status.MarkDisconnected(c.currentConfig(), now)
		case err != nil:
			_ = c.status.MarkFailure(c.currentConfig(), err.Error(), now)
		default:
			_ = c.status.MarkDisconnected(c.currentConfig(), now)
		}
	}()

	challenge, err := c.fetchChallenge(ctx)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.currentConfig(), err.Error(), time.Now().UTC())
		}
		return err
	}
	cfg = c.currentConfig()
	signature, err := SignChallenge(cfg, challenge.ChallengeID, challenge.Nonce)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(cfg, err.Error(), time.Now().UTC())
		}
		return err
	}

	wsURL := challenge.Relay.WebsocketURL
	if wsURL == "" {
		wsURL = cfg.WebsocketURL
	}
	if wsURL == "" {
		wsURL = httpToWebsocket(cfg.RelayURL) + "/ws/connectors"
	}

	conn, _, err := c.dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(cfg, fmt.Errorf("relay unavailable: %w", err).Error(), time.Now().UTC())
		}
		return fmt.Errorf("relay unavailable: %w", err)
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	if err := conn.WriteJSON(relayproto.HelloMessage{
		Type:        "hello",
		ConnectorID: cfg.ConnectorID,
		MachineID:   cfg.MachineID,
		ChallengeID: challenge.ChallengeID,
		Signature:   signature,
	}); err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(cfg, err.Error(), time.Now().UTC())
		}
		return err
	}

	if err := c.advertise(ctx, conn); err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.currentConfig(), err.Error(), time.Now().UTC())
		}
		return err
	}

	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go c.heartbeatLoop(heartbeatCtx, conn)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if c.status != nil && ctx.Err() == nil {
				_ = c.status.MarkFailure(c.currentConfig(), err.Error(), time.Now().UTC())
			}
			return err
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(message, &envelope); err != nil {
			return err
		}
		switch envelope.Type {
		case "request":
			var request relayproto.CommandRequest
			if err := json.Unmarshal(message, &request); err != nil {
				return err
			}
			response := c.handleRequest(ctx, request)
			if err := conn.WriteJSON(response); err != nil {
				return err
			}
		case "error":
			return fmt.Errorf("relay session error")
		default:
			var request relayproto.CommandRequest
			data, _ := json.Marshal(envelope)
			_ = json.Unmarshal(data, &request)
			if request.Type == "request" {
				response := c.handleRequest(ctx, request)
				if err := conn.WriteJSON(response); err != nil {
					return err
				}
			}
		}
	}
}

func (c *Client) fetchChallenge(ctx context.Context) (*relayproto.ChallengeResponse, error) {
	cfg := c.currentConfig()
	payload, err := json.Marshal(relayproto.ChallengeRequest{
		ConnectorID: cfg.ConnectorID,
		MachineID:   cfg.MachineID,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.RelayURL, "/")+"/api/v2/connectors/challenge", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch relay challenge failed: %s", string(body))
	}
	var challenge relayproto.ChallengeResponse
	if err := json.NewDecoder(resp.Body).Decode(&challenge); err != nil {
		return nil, err
	}
	if challenge.Relay.HeartbeatIntervalSeconds > 0 {
		cfg.HeartbeatIntervalSeconds = challenge.Relay.HeartbeatIntervalSeconds
		c.setConfig(cfg)
	}
	return &challenge, nil
}

func (c *Client) advertise(ctx context.Context, conn *websocket.Conn) error {
	cfg := c.currentConfig()
	instances, instanceIDs, err := c.registryForConfig(cfg).Advertisements(ctx)
	if err != nil {
		return err
	}
	if err := conn.WriteJSON(relayproto.AdvertiseMessage{
		Type:      "advertise",
		Instances: instances,
	}); err != nil {
		return err
	}
	if c.status != nil {
		_ = c.status.MarkConnected(cfg, instanceIDs, time.Now().UTC())
	}
	return nil
}

func (c *Client) heartbeatLoop(ctx context.Context, conn *websocket.Conn) {
	lastAdvertisedSignature := sharedSetSignature(c.currentConfig())
	currentInterval := c.heartbeatInterval(c.currentConfig())
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfg, changed, err := c.reloadConfig()
			if err != nil {
				if c.status != nil {
					_ = c.status.MarkFailure(c.currentConfig(), err.Error(), time.Now().UTC())
				}
			}
			nextInterval := c.heartbeatInterval(cfg)
			if nextInterval != currentInterval {
				ticker.Reset(nextInterval)
				currentInterval = nextInterval
			}
			if changed {
				currentSignature := sharedSetSignature(cfg)
				if currentSignature != lastAdvertisedSignature {
					if err := c.advertise(ctx, conn); err != nil {
						if c.status != nil {
							_ = c.status.MarkFailure(c.currentConfig(), err.Error(), time.Now().UTC())
						}
						_ = conn.Close()
						return
					}
					lastAdvertisedSignature = currentSignature
				}
			}

			_, instanceIDs, advertiseErr := c.registryForConfig(cfg).Advertisements(ctx)
			if advertiseErr != nil {
				if c.status != nil {
					_ = c.status.MarkFailure(cfg, advertiseErr.Error(), time.Now().UTC())
				}
				continue
			}
			if err := conn.WriteJSON(relayproto.HeartbeatMessage{
				Type:        "heartbeat",
				ConnectorID: cfg.ConnectorID,
				MachineID:   cfg.MachineID,
				InstanceIDs: instanceIDs,
				SentAt:      time.Now().UTC().Format(time.RFC3339),
			}); err != nil {
				if c.status != nil {
					_ = c.status.MarkFailure(cfg, err.Error(), time.Now().UTC())
				}
				_ = conn.Close()
				return
			}
			if c.status != nil {
				_ = c.status.MarkHeartbeat(cfg, instanceIDs, time.Now().UTC())
			}
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, request relayproto.CommandRequest) relayproto.CommandResponse {
	cfg := c.currentConfig()
	instance, err := c.registryForConfig(cfg).ResolveInstance(ctx, request.InstanceID)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(cfg, err.Error(), time.Now().UTC())
		}
		return relayproto.CommandResponse{
			Type:       "response",
			RequestID:  request.RequestID,
			StatusCode: http.StatusForbidden,
			Error:      err.Error(),
		}
	}
	client := NewCodencerClient(instance.DaemonURL).WithStatusStore(c.status, cfg)
	return client.Proxy(ctx, request)
}

func (c *Client) currentConfig() *Config {
	c.cfgMu.RLock()
	defer c.cfgMu.RUnlock()
	return c.cfg.Clone()
}

func (c *Client) setConfig(cfg *Config) {
	c.cfgMu.Lock()
	defer c.cfgMu.Unlock()
	c.cfg = cfg.Clone()
}

func (c *Client) registryForConfig(cfg *Config) *Registry {
	return NewRegistry(cfg)
}

func (c *Client) reloadConfig() (*Config, bool, error) {
	current := c.currentConfig()
	if current == nil || current.ConfigPath == "" {
		return current, false, nil
	}
	loaded, err := LoadConfig(current.ConfigPath)
	if err != nil {
		return current, false, err
	}
	changed := sharedSetSignature(loaded) != sharedSetSignature(current)
	c.setConfig(loaded)
	return loaded, changed, nil
}

func (c *Client) heartbeatInterval(cfg *Config) time.Duration {
	if cfg == nil {
		return DefaultHeartbeatIntervalSeconds * time.Second
	}
	interval := time.Duration(cfg.HeartbeatIntervalSeconds) * time.Second
	if interval <= 0 {
		return DefaultHeartbeatIntervalSeconds * time.Second
	}
	return interval
}

func sharedSetSignature(cfg *Config) string {
	if cfg == nil {
		return ""
	}
	shared := make([]string, 0, len(cfg.Instances))
	for _, entry := range EffectiveSharedInstances(cfg) {
		if !entry.Share {
			continue
		}
		shared = append(shared, strings.Join([]string{
			entry.InstanceID,
			strings.TrimRight(entry.DaemonURL, "/"),
			entry.ManifestPath,
		}, "|"))
	}
	return strings.Join(shared, "\n")
}

func httpToWebsocket(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	default:
		parsed.Scheme = "ws"
	}
	return strings.TrimRight(parsed.String(), "/")
}
