package service

import (
	"context"
	"fmt"
	"strconv"

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
}

func NewAntigravityService(settingsRepo SettingsStore) *AntigravityService {
	return &AntigravityService{
		settingsRepo: settingsRepo,
		discovery:    antigravity.NewDiscovery(),
	}
}

// ListInstances returns all discovered Antigravity instances.
func (s *AntigravityService) ListInstances(ctx context.Context) ([]domain.AGInstance, error) {
	return s.discovery.Discover(ctx)
}

// Bind links this repo to a specific Antigravity instance by PID.
func (s *AntigravityService) Bind(ctx context.Context, pid int) error {
	return s.settingsRepo.Set(ctx, SettingKeyBoundAGPID, strconv.Itoa(pid))
}

// Unbind clears the binding.
func (s *AntigravityService) Unbind(ctx context.Context) error {
	return s.settingsRepo.Delete(ctx, SettingKeyBoundAGPID)
}

// GetBinding returns the currently bound instance if it is still alive.
func (s *AntigravityService) GetBinding(ctx context.Context) (*domain.AGInstance, error) {
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
