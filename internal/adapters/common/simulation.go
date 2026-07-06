package common

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-bridge/internal/domain"
)

// IsSimulationEnabled checks if an adapter should run in simulation mode.
func IsSimulationEnabled(adapterName string) bool {
	envVar := strings.ToUpper(adapterName) + "_SIMULATION_MODE"
	normalizedEnvVar := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(adapterName)) + "_SIMULATION_MODE"
	return truthySimulationEnv(os.Getenv(envVar)) ||
		truthySimulationEnv(os.Getenv(normalizedEnvVar)) ||
		truthySimulationEnv(os.Getenv("ALL_ADAPTERS_SIMULATION_MODE"))
}

func truthySimulationEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// ResultLooksSimulated detects the deterministic simulation artifact shape.
// Real-executor verifiers rely on is_simulation=false, so adapters must not
// relabel a simulation-produced result as real if environment values change
// between execution and normalization.
func ResultLooksSimulated(adapterName string, result *domain.ResultSpec) bool {
	if result == nil {
		return false
	}
	if result.IsSimulation {
		return true
	}
	adapter := strings.ToLower(strings.TrimSpace(adapterName))
	text := strings.ToLower(strings.TrimSpace(result.Summary + "\n" + result.RawOutput))
	if adapter != "" {
		if strings.Contains(text, "simulated successful "+adapter+" task") {
			return true
		}
		if strings.Contains(text, "simulation: "+adapter+" adapter relay completed successfully") {
			return true
		}
	}
	return strings.Contains(text, "simulation mode: executing stub")
}

// RunSimulation writes stub files to the artifact root to simulate the orchestrator's
// interaction with an adapter. This is used for ORCHESTRATOR LIFECYCLE VERIFICATION ONLY
// and does not execute any real agent logic.
func RunSimulation(ctx context.Context, step *domain.Step, attempt *domain.Attempt, attemptArtifactRoot, workspaceRoot string) error {
	slog.Info("Simulation Mode: Executing stub for attempt", "attemptID", attempt.ID)

	stdoutPath := filepath.Join(attemptArtifactRoot, "stdout.log")
	resultPath := filepath.Join(attemptArtifactRoot, "result.json")

	script := fmt.Sprintf(`
		echo "Executing Simulated %s for attempt %s" > "%s"
		cat << 'EOF' > "%s"
{
  "version": "v1",
  "adapter": "%s",
  "state": "completed",
  "summary": "Simulated successful %s task.",
  "is_simulation": true,
  "needs_human_decision": false
}
EOF
	`, attempt.Adapter, attempt.ID, stdoutPath, resultPath, attempt.Adapter, attempt.Adapter)

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Dir = workspaceRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("simulated execution failed: %w. Output: %s", err, string(out))
	}
	return nil
}
