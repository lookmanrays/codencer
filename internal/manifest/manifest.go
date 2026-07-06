package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "codencer.io/v1alpha1"
	Kind       = "RunManifest"
)

type Manifest struct {
	Version   string    `json:"version" yaml:"version"`
	Kind      string    `json:"kind" yaml:"kind"`
	Metadata  Metadata  `json:"metadata" yaml:"metadata"`
	Project   Project   `json:"project" yaml:"project"`
	Execution Execution `json:"execution" yaml:"execution"`
	Policy    Policy    `json:"policy" yaml:"policy"`
	Tasks     []Task    `json:"tasks" yaml:"tasks"`
}

type Metadata struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

type Project struct {
	ID string `json:"id" yaml:"id"`
}

type Execution struct {
	Adapter string `json:"adapter" yaml:"adapter"`
	Profile string `json:"profile" yaml:"profile"`
	Timeout string `json:"timeout" yaml:"timeout"`
}

type Policy struct {
	StopOnBlocker *bool       `json:"stop_on_blocker" yaml:"stop_on_blocker"`
	StopOnFailure *bool       `json:"stop_on_failure" yaml:"stop_on_failure"`
	Retry         RetryPolicy `json:"retry" yaml:"retry"`
}

type RetryPolicy struct {
	Enabled     bool `json:"enabled" yaml:"enabled"`
	MaxAttempts int  `json:"max_attempts" yaml:"max_attempts"`
}

type Task struct {
	ID          string                     `json:"id" yaml:"id"`
	Title       string                     `json:"title" yaml:"title"`
	Goal        string                     `json:"goal" yaml:"goal"`
	Prompt      string                     `json:"prompt" yaml:"prompt"`
	PromptFile  string                     `json:"prompt_file" yaml:"prompt_file"`
	Adapter     string                     `json:"adapter" yaml:"adapter"`
	Profile     string                     `json:"profile" yaml:"profile"`
	Timeout     string                     `json:"timeout" yaml:"timeout"`
	Validations []domain.ValidationCommand `json:"validations" yaml:"validations"`
	DependsOn   []string                   `json:"depends_on" yaml:"depends_on"`
}

func Load(path string) (*Manifest, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read manifest: %w", err)
	}
	manifest, err := Parse(data)
	if err != nil {
		return nil, data, err
	}
	return manifest, data, nil
}

func Parse(data []byte) (*Manifest, error) {
	var manifest Manifest
	if json.Valid(data) {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parse manifest json: %w", err)
		}
	} else if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest yaml: %w", err)
	}
	ApplyDefaults(&manifest)
	if err := Validate(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func ApplyDefaults(manifest *Manifest) {
	if manifest == nil {
		return
	}
	if manifest.Policy.StopOnBlocker == nil {
		value := true
		manifest.Policy.StopOnBlocker = &value
	}
	if manifest.Policy.StopOnFailure == nil {
		value := true
		manifest.Policy.StopOnFailure = &value
	}
	if manifest.Policy.Retry.MaxAttempts <= 0 {
		manifest.Policy.Retry.MaxAttempts = 1
	}
}

func Validate(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is required")
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("manifest version is required")
	}
	if manifest.Version != APIVersion {
		return fmt.Errorf("unsupported manifest version %q", manifest.Version)
	}
	if strings.TrimSpace(manifest.Kind) == "" {
		return fmt.Errorf("manifest kind is required")
	}
	if manifest.Kind != Kind {
		return fmt.Errorf("unsupported manifest kind %q", manifest.Kind)
	}
	if len(manifest.Tasks) == 0 {
		return fmt.Errorf("manifest tasks are required")
	}
	seen := map[string]struct{}{}
	for i, task := range manifest.Tasks {
		if strings.TrimSpace(task.ID) == "" {
			return fmt.Errorf("task %d id is required", i+1)
		}
		if _, ok := seen[task.ID]; ok {
			return fmt.Errorf("duplicate task id %q", task.ID)
		}
		seen[task.ID] = struct{}{}
		if strings.TrimSpace(task.Goal) == "" && strings.TrimSpace(task.Prompt) == "" && strings.TrimSpace(task.PromptFile) == "" {
			return fmt.Errorf("task %q requires goal, prompt, or prompt_file", task.ID)
		}
		if len(task.DependsOn) > 0 {
			return fmt.Errorf("task %q uses depends_on; Sprint 2 supports sequential manifests only", task.ID)
		}
		if _, err := TimeoutSeconds(firstNonEmpty(task.Timeout, manifest.Execution.Timeout)); err != nil {
			return fmt.Errorf("task %q timeout: %w", task.ID, err)
		}
	}
	return nil
}

func ProjectID(cliProjectID string, manifest *Manifest) string {
	if strings.TrimSpace(cliProjectID) != "" {
		return strings.TrimSpace(cliProjectID)
	}
	if manifest == nil {
		return ""
	}
	return strings.TrimSpace(manifest.Project.ID)
}

func StopOnBlocker(policy Policy) bool {
	if policy.StopOnBlocker == nil {
		return true
	}
	return *policy.StopOnBlocker
}

func StopOnFailure(policy Policy) bool {
	if policy.StopOnFailure == nil {
		return true
	}
	return *policy.StopOnFailure
}

func TimeoutSeconds(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return int(duration.Seconds()), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
