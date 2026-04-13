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
	"time"

	"agent-bridge/internal/relayproto"
	"github.com/gorilla/websocket"
)

type Client struct {
	cfg        *Config
	httpClient *http.Client
	registry   *Registry
	status     *StatusStore
	backoff    *Backoff
	dialer     *websocket.Dialer
}

func NewClient(cfg *Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		registry:   NewRegistry(cfg),
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
	if c.status != nil {
		_ = c.status.MarkConnecting(c.cfg)
	}
	defer func() {
		if c.status == nil {
			return
		}
		now := time.Now().UTC()
		switch {
		case ctx.Err() != nil:
			_ = c.status.MarkDisconnected(c.cfg, now)
		case err != nil:
			_ = c.status.MarkFailure(c.cfg, err.Error(), now)
		default:
			_ = c.status.MarkDisconnected(c.cfg, now)
		}
	}()

	challenge, err := c.fetchChallenge(ctx)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
		}
		return err
	}
	signature, err := SignChallenge(c.cfg, challenge.ChallengeID, challenge.Nonce)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
		}
		return err
	}

	wsURL := challenge.Relay.WebsocketURL
	if wsURL == "" {
		wsURL = c.cfg.WebsocketURL
	}
	if wsURL == "" {
		wsURL = httpToWebsocket(c.cfg.RelayURL) + "/ws/connectors"
	}

	conn, _, err := c.dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.cfg, fmt.Errorf("relay unavailable: %w", err).Error(), time.Now().UTC())
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
		ConnectorID: c.cfg.ConnectorID,
		MachineID:   c.cfg.MachineID,
		ChallengeID: challenge.ChallengeID,
		Signature:   signature,
	}); err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
		}
		return err
	}

	if err := c.advertise(ctx, conn); err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
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
				_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
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
	payload, err := json.Marshal(relayproto.ChallengeRequest{
		ConnectorID: c.cfg.ConnectorID,
		MachineID:   c.cfg.MachineID,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.RelayURL, "/")+"/api/v2/connectors/challenge", bytes.NewReader(payload))
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
		c.cfg.HeartbeatIntervalSeconds = challenge.Relay.HeartbeatIntervalSeconds
	}
	return &challenge, nil
}

func (c *Client) advertise(ctx context.Context, conn *websocket.Conn) error {
	instances, instanceIDs, err := c.registry.Advertisements(ctx)
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
		_ = c.status.MarkConnected(c.cfg, instanceIDs, time.Now().UTC())
	}
	return nil
}

func (c *Client) heartbeatLoop(ctx context.Context, conn *websocket.Conn) {
	interval := time.Duration(c.cfg.HeartbeatIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = DefaultHeartbeatIntervalSeconds * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, instanceIDs, err := c.registry.Advertisements(ctx)
			if err != nil {
				if c.status != nil {
					_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
				}
				continue
			}
			if err := conn.WriteJSON(relayproto.HeartbeatMessage{
				Type:        "heartbeat",
				ConnectorID: c.cfg.ConnectorID,
				MachineID:   c.cfg.MachineID,
				InstanceIDs: instanceIDs,
				SentAt:      time.Now().UTC().Format(time.RFC3339),
			}); err != nil {
				if c.status != nil {
					_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
				}
				_ = conn.Close()
				return
			}
			if c.status != nil {
				_ = c.status.MarkHeartbeat(c.cfg, instanceIDs, time.Now().UTC())
			}
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, request relayproto.CommandRequest) relayproto.CommandResponse {
	instance, err := c.registry.ResolveInstance(ctx, request.InstanceID)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(c.cfg, err.Error(), time.Now().UTC())
		}
		return relayproto.CommandResponse{
			Type:       "response",
			RequestID:  request.RequestID,
			StatusCode: http.StatusForbidden,
			Error:      err.Error(),
		}
	}
	client := NewCodencerClient(instance.DaemonURL).WithStatusStore(c.status, c.cfg)
	return client.Proxy(ctx, request)
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
