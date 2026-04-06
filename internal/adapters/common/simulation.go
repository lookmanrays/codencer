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
	return os.Getenv(envVar) == "1" || os.Getenv("ALL_ADAPTERS_SIMULATION_MODE") == "1"
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
  "state": "completed",
  "summary": "Simulated successful %s task.",
  "needs_human_decision": false
}
EOF
	`, attempt.Adapter, attempt.ID, stdoutPath, resultPath, attempt.Adapter)

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Dir = workspaceRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("simulated execution failed: %w. Output: %s", err, string(out))
	}
	return nil
}
