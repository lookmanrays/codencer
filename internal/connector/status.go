package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	SessionStateDisconnected = "disconnected"
	SessionStateConnecting   = "connecting"
	SessionStateConnected    = "connected"
	SessionStateError        = "error"
)

type Status struct {
	ConnectorID      string   `json:"connector_id,omitempty"`
	MachineID        string   `json:"machine_id,omitempty"`
	RelayURL         string   `json:"relay_url,omitempty"`
	SessionState     string   `json:"session_state"`
	LastConnectAt    string   `json:"last_connect_at,omitempty"`
	LastDisconnectAt string   `json:"last_disconnect_at,omitempty"`
	LastHeartbeatAt  string   `json:"last_heartbeat_at,omitempty"`
	LastError        string   `json:"last_error,omitempty"`
	SharedInstances  []string `json:"shared_instances,omitempty"`
}

type StatusStore struct {
	path string
	mu   sync.Mutex
}

func NewStatusStore(configPath string) *StatusStore {
	return &StatusStore{path: StatusPathForConfig(configPath)}
}

func LoadStatus(configPath string) (*Status, error) {
	path := StatusPathForConfig(configPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read connector status: %w", err)
	}
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("decode connector status: %w", err)
	}
	return &status, nil
}

func (s *StatusStore) Seed(cfg *Config) error {
	return s.update(func(status *Status) {
		seedStatus(status, cfg)
		if status.SessionState == "" {
			status.SessionState = SessionStateDisconnected
		}
		status.LastError = ""
	})
}

func (s *StatusStore) MarkConnecting(cfg *Config) error {
	return s.update(func(status *Status) {
		seedStatus(status, cfg)
		status.SessionState = SessionStateConnecting
		status.LastError = ""
	})
}

func (s *StatusStore) MarkConnected(cfg *Config, sharedInstances []string, now time.Time) error {
	return s.update(func(status *Status) {
		seedStatus(status, cfg)
		status.SessionState = SessionStateConnected
		status.LastConnectAt = formatStatusTime(now)
		status.SharedInstances = uniqueStrings(sharedInstances)
		status.LastError = ""
	})
}

func (s *StatusStore) MarkHeartbeat(cfg *Config, sharedInstances []string, now time.Time) error {
	return s.update(func(status *Status) {
		seedStatus(status, cfg)
		status.SessionState = SessionStateConnected
		status.LastHeartbeatAt = formatStatusTime(now)
		status.SharedInstances = uniqueStrings(sharedInstances)
	})
}

func (s *StatusStore) MarkDisconnected(cfg *Config, now time.Time) error {
	return s.update(func(status *Status) {
		seedStatus(status, cfg)
		status.SessionState = SessionStateDisconnected
		status.LastDisconnectAt = formatStatusTime(now)
		status.LastError = ""
	})
}

func (s *StatusStore) MarkFailure(cfg *Config, err string, now time.Time) error {
	return s.update(func(status *Status) {
		seedStatus(status, cfg)
		status.SessionState = SessionStateError
		status.LastDisconnectAt = formatStatusTime(now)
		status.LastError = err
	})
}

func (s *StatusStore) update(mutator func(*Status)) error {
	if s == nil || s.path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	status, err := loadStatusFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		status = &Status{}
	}
	mutator(status)
	return saveStatusFile(s.path, status)
}

func loadStatusFile(path string) (*Status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func saveStatusFile(path string, status *Status) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".status-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func seedStatus(status *Status, cfg *Config) {
	if status == nil || cfg == nil {
		return
	}
	if cfg.ConnectorID != "" {
		status.ConnectorID = cfg.ConnectorID
	}
	if cfg.MachineID != "" {
		status.MachineID = cfg.MachineID
	}
	if cfg.RelayURL != "" {
		status.RelayURL = cfg.RelayURL
	}
	status.SharedInstances = sharedInstanceIDs(cfg)
}

func sharedInstanceIDs(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(cfg.Instances))
	for _, inst := range cfg.Instances {
		if !inst.Share {
			continue
		}
		id := inst.InstanceID
		if id == "" {
			id = inst.DaemonURL
		}
		if id == "" {
			id = inst.ManifestPath
		}
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func formatStatusTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
