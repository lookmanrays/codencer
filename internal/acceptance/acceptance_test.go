package acceptance

import (
	"testing"

	"agent-bridge/internal/live"
)

func TestVerdictReadyWithSkips(t *testing.T) {
	report := Report{
		Gates: []Gate{
			{ID: "required", Status: live.StatusPassed, Required: true},
			{ID: "codex", Status: live.StatusSkipped, Required: false, Live: true},
		},
	}
	report.computeVerdict()
	if !report.OK || report.Verdict != VerdictReadyWithSkips {
		t.Fatalf("unexpected verdict: %+v", report)
	}
	if len(report.SkippedLiveGates) != 1 || report.SkippedLiveGates[0] != "codex" {
		t.Fatalf("expected skipped live gate: %+v", report.SkippedLiveGates)
	}
}

func TestVerdictBlocksOnRequiredFailure(t *testing.T) {
	report := Report{Gates: []Gate{{ID: "local", Status: live.StatusFailed, Required: true}}}
	report.computeVerdict()
	if report.OK || report.Verdict != VerdictNotReady {
		t.Fatalf("unexpected verdict: %+v", report)
	}
	if len(report.BlockingGates) != 1 || report.BlockingGates[0] != "local" {
		t.Fatalf("expected blocking gate: %+v", report.BlockingGates)
	}
}
