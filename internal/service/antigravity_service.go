package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"agent-bridge/internal/adapters/antigravity"
	"agent-bridge/internal/domain"
)

// SettingsStore defines the interface for persisting repo-local settings.
type SettingsStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

const (
	SettingKeyBoundAGPID = "bound_ag_pid"
)

// AntigravityService manages discovery and binding of Antigravity executors.
type AntigravityService struct {
	settingsRepo SettingsStore
	discovery    *antigravity.Discovery
	brokerURL    string
	httpClient   *http.Client
}

func NewAntigravityService(settingsRepo SettingsStore, brokerURL string) *AntigravityService {
	svc := &AntigravityService{
		settingsRepo: settingsRepo,
		discovery:    antigravity.NewDiscovery(),
		brokerURL:    brokerURL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	if svc.isBrokerEnabled() {
		slog.Info("AntigravityService initialized in BROKER mode", "url", brokerURL)
	} else {
		slog.Info("AntigravityService initialized in DIRECT mode (experimental for WSL)")
	}

	return svc
}

func (s *AntigravityService) isBrokerEnabled() bool {
	return s.brokerURL != ""
}

// ListInstances returns all discovered Antigravity instances.
func (s *AntigravityService) ListInstances(ctx context.Context) ([]domain.AGInstance, error) {
	if s.isBrokerEnabled() {
		resp, err := s.httpClient.Get(s.brokerURL + "/instances")
		if err != nil {
			return nil, fmt.Errorf("broker error: %w", err)
		}
		defer resp.Body.Close()
		var instances []domain.AGInstance
		if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
			return nil, err
		}
		return instances, nil
	}
	return s.discovery.Discover(ctx)
}

// Bind links this repo to a specific Antigravity instance by PID.
func (s *AntigravityService) Bind(ctx context.Context, pid int) error {
	if s.isBrokerEnabled() {
		data, _ := json.Marshal(map[string]int{"pid": pid})
		resp, err := s.httpClient.Post(s.brokerURL+"/binding", "application/json", bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("broker bind error: %w (check if host-side broker is running on port 8088)", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("broker bind failed (%d): %s", resp.StatusCode, string(body))
		}
		return nil
	}
	return s.settingsRepo.Set(ctx, SettingKeyBoundAGPID, strconv.Itoa(pid))
}

// Unbind clears the binding.
func (s *AntigravityService) Unbind(ctx context.Context) error {
	if s.isBrokerEnabled() {
		req, _ := http.NewRequestWithContext(ctx, "DELETE", s.brokerURL+"/binding", nil)
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("broker unbind error: %w", err)
		}
		defer resp.Body.Close()
		return nil
	}
	return s.settingsRepo.Delete(ctx, SettingKeyBoundAGPID)
}

// GetBinding returns the currently bound instance if it is still alive.
func (s *AntigravityService) GetBinding(ctx context.Context) (*domain.AGInstance, error) {
	if s.isBrokerEnabled() {
		resp, err := s.httpClient.Get(s.brokerURL + "/binding")
		if err != nil {
			return nil, fmt.Errorf("broker get binding error: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		
		var inst domain.AGInstance
		if err := json.NewDecoder(resp.Body).Decode(&inst); err != nil {
			return nil, nil // Assume unbound if JSON is malformed or "unbound" status
		}
		if inst.PID == 0 {
			return nil, nil
		}
		return &inst, nil
	}

	pidStr, err := s.settingsRepo.Get(ctx, SettingKeyBoundAGPID)
	if err != nil {
		return nil, err
	}
	if pidStr == "" {
		return nil, nil
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid bound PID %q: %w", pidStr, err)
	}

	instances, err := s.discovery.Discover(ctx)
	if err != nil {
		return nil, err
	}

	for _, inst := range instances {
		if inst.PID == pid {
			return &inst, nil
		}
	}

	return &domain.AGInstance{PID: pid, IsReachable: false}, nil
}
