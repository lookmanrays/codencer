package workspace

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"agent-bridge/internal/domain"
	"gopkg.in/yaml.v3"
)

// LoadWorkspaceConfig probes the repository for provisioning configuration.
// It prioritizes:
// 1. .codencer/workspace.json (Native)
// 2. grove.yaml (Spec-Grove)
// 3. .groverc.json (Legacy-Grove)
func LoadWorkspaceConfig(repoRoot string) (*domain.ProvisioningSpec, error) {
	spec := &domain.ProvisioningSpec{}
	
	// 1. Native .codencer/workspace.json
	nativePath := filepath.Join(repoRoot, ".codencer", "workspace.json")
	if data, err := os.ReadFile(nativePath); err == nil {
		var wrapper struct {
			Provisioning domain.ProvisioningSpec `json:"provisioning"`
		}
		if err := json.Unmarshal(data, &wrapper); err != nil {
			slog.Warn("Failed to parse .codencer/workspace.json", "error", err)
		} else {
			*spec = wrapper.Provisioning
		}
	}

	// 2. Spec-Grove (grove.yaml)
	if groveData, err := os.ReadFile(filepath.Join(repoRoot, "grove.yaml")); err == nil {
			var grove struct {
				Workspace struct {
					Setup struct {
						Copy     []string `yaml:"copy"`
						Symlinks []string `yaml:"symlinks"`
					} `yaml:"setup"`
					Hooks struct {
						PostCreate string `yaml:"post_create"`
					} `yaml:"hooks"`
				} `yaml:"workspace"`
			}
			if err := yaml.Unmarshal(groveData, &grove); err != nil {
				slog.Warn("Failed to parse grove.yaml", "error", err)
			} else {
				// Fallback merge
				if len(spec.Copy) == 0 { spec.Copy = grove.Workspace.Setup.Copy }
				if len(spec.Symlinks) == 0 { spec.Symlinks = grove.Workspace.Setup.Symlinks }
				if spec.Hooks.PostCreate == "" { spec.Hooks.PostCreate = grove.Workspace.Hooks.PostCreate }
			}
	}

	// 3. Legacy-Grove (.groverc.json - reference repo style)
	if legacyData, err := os.ReadFile(filepath.Join(repoRoot, ".groverc.json")); err == nil {
		var legacy struct {
			Symlink     []string `json:"symlink"`
			AfterCreate string   `json:"afterCreate"`
		}
		if err := json.Unmarshal(legacyData, &legacy); err != nil {
			slog.Warn("Failed to parse .groverc.json", "error", err)
		} else {
			// Fallback merge
			if len(spec.Symlinks) == 0 { spec.Symlinks = legacy.Symlink }
			if spec.Hooks.PostCreate == "" { spec.Hooks.PostCreate = legacy.AfterCreate }
		}
	}

	// Only return a spec if it actually defines actions to take
	if len(spec.Copy) == 0 && len(spec.Symlinks) == 0 && spec.Hooks.PostCreate == "" {
		return nil, nil
	}
	
	return spec, nil
}
