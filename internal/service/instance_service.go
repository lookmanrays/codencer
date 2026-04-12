package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"agent-bridge/internal/domain"
)

const instanceIDSettingKey = "daemon_instance_id"

// InstanceService owns stable daemon identity, manifest writing, and runtime capability reporting.
type InstanceService struct {
	settingsRepo  SettingsStore
	agSvc         *AntigravityService
	version       string
	startedAt     time.Time
	repoRoot      string
	stateDir      string
	workspaceRoot string
	host          string
	port          int
	instanceID    string
	getAdapters   func() map[string]domain.Adapter
}

func NewInstanceService(
	settingsRepo SettingsStore,
	agSvc *AntigravityService,
	version string,
	startedAt time.Time,
	repoRoot string,
	stateDir string,
	workspaceRoot string,
	host string,
	port int,
	getAdapters func() map[string]domain.Adapter,
) *InstanceService {
	return &InstanceService{
		settingsRepo:  settingsRepo,
		agSvc:         agSvc,
		version:       version,
		startedAt:     startedAt,
		repoRoot:      repoRoot,
		stateDir:      stateDir,
		workspaceRoot: workspaceRoot,
		host:          host,
		port:          port,
		getAdapters:   getAdapters,
	}
}

func (s *InstanceService) EnsureStableInstanceID(ctx context.Context) (string, error) {
	if s.instanceID != "" {
		return s.instanceID, nil
	}
	existing, err := s.settingsRepo.Get(ctx, instanceIDSettingKey)
	if err != nil {
		return "", err
	}
	if existing != "" {
		s.instanceID = existing
		return existing, nil
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	instanceID := "inst-" + hex.EncodeToString(buf)
	if err := s.settingsRepo.Set(ctx, instanceIDSettingKey, instanceID); err != nil {
		return "", err
	}
	s.instanceID = instanceID
	return instanceID, nil
}

func (s *InstanceService) ManifestPath() string {
	return filepath.Join(s.stateDir, "instance.json")
}

func (s *InstanceService) Compatibility(ctx context.Context) domain.CompatibilityInfo {
	environment := domain.CompatibilityEnvironment{
		OS:             runtime.GOOS,
		VSCodeDetected: os.Getenv("VSCODE_PID") != "" || strings.EqualFold(os.Getenv("TERM_PROGRAM"), "vscode"),
	}

	adaptersMap := s.getAdaptersSnapshot()
	adapterIDs := make([]string, 0, len(adaptersMap))
	for id := range adaptersMap {
		adapterIDs = append(adapterIDs, id)
	}
	sort.Strings(adapterIDs)

	adapters := make([]domain.CompatibilityAdapter, 0, len(adapterIDs))
	tier := 0
	for _, id := range adapterIDs {
		mode, available, status := s.adapterRuntimeState(ctx, id, environment)
		if available && tier < 2 {
			tier = 2
		}
		if id == "ide-chat" && available && tier < 3 {
			tier = 3
		}
		adapters = append(adapters, domain.CompatibilityAdapter{
			ID:           id,
			Available:    available,
			Status:       status,
			Mode:         mode,
			Capabilities: adaptersMap[id].Capabilities(),
		})
	}

	return domain.CompatibilityInfo{
		Tier:        tier,
		Adapters:    adapters,
		Environment: environment,
	}
}

func (s *InstanceService) Current(ctx context.Context) (domain.InstanceInfo, error) {
	instanceID, err := s.EnsureStableInstanceID(ctx)
	if err != nil {
		return domain.InstanceInfo{}, err
	}

	compatibility := s.Compatibility(ctx)
	brokerInfo := domain.InstanceBrokerInfo{Mode: "direct"}
	if s.agSvc != nil {
		info, err := s.agSvc.BrokerInfo(ctx)
		if err != nil {
			return domain.InstanceInfo{}, err
		}
		brokerInfo = info
	}

	return domain.InstanceInfo{
		ID:            instanceID,
		Version:       s.version,
		RepoName:      filepath.Base(s.repoRoot),
		RepoRoot:      s.repoRoot,
		StateDir:      s.stateDir,
		WorkspaceRoot: s.workspaceRoot,
		ManifestPath:  s.ManifestPath(),
		Host:          s.host,
		Port:          s.port,
		BaseURL:       fmt.Sprintf("http://%s:%d", s.host, s.port),
		ExecutionMode: s.executionMode(),
		PID:           os.Getpid(),
		StartedAt:     s.startedAt,
		Adapters:      compatibility.Adapters,
		Broker:        brokerInfo,
	}, nil
}

func (s *InstanceService) WriteManifest(ctx context.Context) (string, error) {
	info, err := s.Current(ctx)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(s.ManifestPath()), 0755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	tmpPath := s.ManifestPath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, s.ManifestPath()); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return s.ManifestPath(), nil
}

func (s *InstanceService) getAdaptersSnapshot() map[string]domain.Adapter {
	if s.getAdapters == nil {
		return map[string]domain.Adapter{}
	}
	adapters := s.getAdapters()
	if adapters == nil {
		return map[string]domain.Adapter{}
	}
	return adapters
}

func (s *InstanceService) executionMode() string {
	value := os.Getenv("ALL_ADAPTERS_SIMULATION_MODE")
	if value == "1" || strings.EqualFold(value, "true") {
		return "simulation"
	}
	return "real"
}

func (s *InstanceService) adapterRuntimeState(ctx context.Context, id string, environment domain.CompatibilityEnvironment) (string, bool, string) {
	if isSimulationMode(id) {
		return "simulation", true, "simulation"
	}

	switch id {
	case "codex":
		return binaryRuntimeState("CODEX_BINARY", "codex-agent")
	case "claude":
		return binaryRuntimeState("CLAUDE_BINARY", "claude")
	case "qwen":
		return binaryRuntimeState("QWEN_BINARY", "qwen-local")
	case "openclaw-acpx":
		return binaryRuntimeState("OPENCLAW_ACPX_BINARY", "acpx")
	case "ide-chat":
		if environment.VSCodeDetected {
			return "real", true, "available"
		}
		return "unavailable", false, "missing_vscode"
	case "antigravity":
		if s.agSvc == nil {
			return "real", false, "unknown_binding"
		}
		binding, err := s.agSvc.GetBinding(ctx)
		if err != nil {
			return "real", false, "binding_error"
		}
		if binding == nil {
			return "real", false, "unbound"
		}
		if binding.IsReachable {
			return "real", true, "bound"
		}
		return "real", false, "bound_unreachable"
	case "antigravity-broker":
		if s.agSvc != nil && s.agSvc.BrokerEnabled() {
			return "real", true, "configured"
		}
		return "unavailable", false, "disabled"
	default:
		return "real", true, "registered"
	}
}

func binaryRuntimeState(envVar, fallback string) (string, bool, string) {
	binary := os.Getenv(envVar)
	if binary == "" {
		binary = fallback
	}
	if _, err := exec.LookPath(binary); err != nil {
		return "unavailable", false, "missing_binary"
	}
	return "real", true, "available"
}

func isSimulationMode(adapterID string) bool {
	if value := os.Getenv("ALL_ADAPTERS_SIMULATION_MODE"); value == "1" || strings.EqualFold(value, "true") {
		return true
	}
	value := os.Getenv(strings.ToUpper(strings.ReplaceAll(adapterID, "-", "_")) + "_SIMULATION_MODE")
	return value == "1" || strings.EqualFold(value, "true")
}
