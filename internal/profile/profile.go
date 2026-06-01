package profile

import (
	"fmt"
	"sort"
	"strings"
)

const DangerousBypassEnv = "CODENCER_ALLOW_DANGEROUS_BYPASS"

type Profile struct {
	ID                                   string `json:"id"`
	Adapter                              string `json:"adapter"`
	DaemonAdapter                        string `json:"daemon_adapter"`
	Approval                             string `json:"approval,omitempty"`
	Sandbox                              string `json:"sandbox,omitempty"`
	OutputFormat                         string `json:"output_format,omitempty"`
	DangerousBypassApprovalsAndSandbox   bool   `json:"dangerous_bypass_approvals_and_sandbox,omitempty"`
	RequiresExplicitAllowDangerousBypass bool   `json:"requires_explicit_allow_dangerous_bypass,omitempty"`
	Description                          string `json:"description"`
}

type ResolveOptions struct {
	ProfileID             string
	Adapter               string
	ProjectDefaultAdapter string
	ProjectProfile        string
	AllowDangerousBypass  bool
}

type Resolution struct {
	Profile       Profile `json:"profile"`
	ProfileID     string  `json:"profile_id"`
	Adapter       string  `json:"adapter"`
	DaemonAdapter string  `json:"daemon_adapter"`
	Source        string  `json:"source"`
}

func Builtins() map[string]Profile {
	return map[string]Profile{
		"codex-workspace": {
			ID:            "codex-workspace",
			Adapter:       "codex",
			DaemonAdapter: "codex",
			Approval:      "never",
			Sandbox:       "workspace-write",
			Description:   "Non-interactive Codex execution with workspace write access.",
		},
		"codex-full": {
			ID:            "codex-full",
			Adapter:       "codex",
			DaemonAdapter: "codex",
			Approval:      "never",
			Sandbox:       "danger-full-access",
			Description:   "Non-interactive Codex execution with full local access. Requires explicit profile selection.",
		},
		"codex-danger-bypass": {
			ID:                                   "codex-danger-bypass",
			Adapter:                              "codex",
			DaemonAdapter:                        "codex",
			DangerousBypassApprovalsAndSandbox:   true,
			RequiresExplicitAllowDangerousBypass: true,
			Description:                          "Maximum bypass mode. Only for isolated machines, VMs, or containers.",
		},
		"claude-default": {
			ID:            "claude-default",
			Adapter:       "claude",
			DaemonAdapter: "claude",
			OutputFormat:  "json",
			Description:   "Non-interactive Claude CLI execution.",
		},
		"fake-success": {
			ID:            "fake-success",
			Adapter:       "fake",
			DaemonAdapter: "fake-success",
			Description:   "Deterministic test success profile.",
		},
		"fake-failure": {
			ID:            "fake-failure",
			Adapter:       "fake",
			DaemonAdapter: "fake-failure",
			Description:   "Deterministic test terminal failure profile.",
		},
		"fake-blocker": {
			ID:            "fake-blocker",
			Adapter:       "fake",
			DaemonAdapter: "fake-blocker",
			Description:   "Deterministic test blocker profile.",
		},
		"fake-timeout": {
			ID:            "fake-timeout",
			Adapter:       "fake",
			DaemonAdapter: "fake-timeout",
			Description:   "Deterministic test timeout profile.",
		},
	}
}

func List() []Profile {
	profiles := Builtins()
	ids := make([]string, 0, len(profiles))
	for id := range profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Profile, 0, len(ids))
	for _, id := range ids {
		out = append(out, profiles[id])
	}
	return out
}

func Get(id string) (Profile, bool) {
	p, ok := Builtins()[strings.TrimSpace(id)]
	return p, ok
}

func Resolve(opts ResolveOptions) (Resolution, error) {
	profileID, source := firstNonEmptyWithSource(
		valueWithSource{value: opts.ProfileID, source: "explicit_profile"},
		valueWithSource{value: opts.ProjectProfile, source: "project_profile"},
	)

	adapter := strings.TrimSpace(opts.Adapter)
	if adapter == "" {
		adapter = strings.TrimSpace(opts.ProjectDefaultAdapter)
	}

	if profileID == "" {
		profileID = defaultProfileForAdapter(adapter)
		source = "adapter_default"
	} else if _, ok := Get(profileID); !ok {
		// Sprint 1 stored adapter ids in adapter_profile when no profile was supplied.
		if mapped := defaultProfileForAdapter(profileID); mapped != "" {
			profileID = mapped
			source = "legacy_adapter_profile"
		}
	}

	if profileID == "" {
		return Resolution{}, fmt.Errorf("adapter profile is required")
	}
	p, ok := Get(profileID)
	if !ok {
		return Resolution{}, fmt.Errorf("unknown adapter profile %q", profileID)
	}
	if adapter != "" && p.Adapter != adapter && p.DaemonAdapter != adapter {
		return Resolution{}, fmt.Errorf("profile %q is for adapter %q, not %q", p.ID, p.Adapter, adapter)
	}
	if p.RequiresExplicitAllowDangerousBypass && !opts.AllowDangerousBypass {
		return Resolution{}, fmt.Errorf("profile %q requires %s=1", p.ID, DangerousBypassEnv)
	}
	return Resolution{
		Profile:       p,
		ProfileID:     p.ID,
		Adapter:       p.Adapter,
		DaemonAdapter: p.DaemonAdapter,
		Source:        source,
	}, nil
}

func defaultProfileForAdapter(adapter string) string {
	switch strings.TrimSpace(adapter) {
	case "codex":
		return "codex-workspace"
	case "claude":
		return "claude-default"
	case "fake", "fake-success", "fake-failure", "fake-blocker", "fake-timeout":
		if strings.HasPrefix(adapter, "fake-") {
			return adapter
		}
		return "fake-success"
	default:
		return ""
	}
}

type valueWithSource struct {
	value  string
	source string
}

func firstNonEmptyWithSource(values ...valueWithSource) (string, string) {
	for _, candidate := range values {
		if strings.TrimSpace(candidate.value) != "" {
			return strings.TrimSpace(candidate.value), candidate.source
		}
	}
	return "", ""
}
