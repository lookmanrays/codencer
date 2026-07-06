package cliui

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIsInteractiveGuardsMachineReadableOutput(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "json", opts: Options{JSON: true, ForceInteractive: true, Stderr: os.Stderr}},
		{name: "ci", opts: Options{CI: true, ForceInteractive: true, Stderr: os.Stderr}},
		{name: "no animation", opts: Options{NoAnimation: true, ForceInteractive: true, Stderr: os.Stderr}},
		{name: "no color", opts: Options{NoColor: true, ForceInteractive: true, Stderr: os.Stderr}},
		{name: "non tty writer", opts: Options{Stderr: &bytes.Buffer{}}},
		{name: "non tty stdout", opts: Options{Stdout: &bytes.Buffer{}, Stderr: os.Stderr}},
		{name: "non tty stderr", opts: Options{Stdout: os.Stdout, Stderr: &bytes.Buffer{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsInteractive(tt.opts) {
				t.Fatalf("expected %s to be non-interactive", tt.name)
			}
		})
	}
}

func TestMarkFrameRestingUsesCompactMarkAndRestingBars(t *testing.T) {
	got := MarkFrame(0, false, true)
	want := ansiOrange + "█" + ansiFGReset + "▌" + ansiOrange + "▊" + ansiFGReset + "▎"
	if got != want {
		t.Fatalf("resting mark mismatch:\ngot  %q\nwant %q", got, want)
	}
	if plain := MarkFrame(0, false, false); plain != TerminalMarkCompact {
		t.Fatalf("plain mark = %q want %q", plain, TerminalMarkCompact)
	}
}

func TestWorkingMarkNeverHasZeroLitBars(t *testing.T) {
	for bucket := 0; bucket < 200; bucket++ {
		got := MarkFrame(bucket, true, true)
		if count := strings.Count(got, ansiOrange); count == 0 {
			t.Fatalf("bucket %d had no lit bars: %q", bucket, got)
		}
	}
}

func TestSpinnerFrameCyclesThroughReferenceGlyphs(t *testing.T) {
	want := "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	var got strings.Builder
	for i := 0; i < len(spinnerGlyphs); i++ {
		got.WriteString(SpinnerFrame(i))
	}
	if got.String() != want {
		t.Fatalf("spinner cycle = %q want %q", got.String(), want)
	}
	if SpinnerFrame(len(spinnerGlyphs)) != "⠋" {
		t.Fatalf("spinner should wrap at cycle length")
	}
}

func TestRenderWorkingFrameTasks(t *testing.T) {
	rows := RenderWorkingFrame(2, 1900*time.Millisecond, []string{"read schema", "plan diff", "apply patch", "run tests"}, RenderOptions{
		Label:  "codencer",
		Color:  false,
		StepMs: 900 * time.Millisecond,
	})
	joined := strings.Join(rows, "\n")
	for _, want := range []string{
		"codencer   ⠹ running",
		"  ✓ 01  read schema",
		"  ✓ 02  plan diff",
		"  ⠹ 03  apply patch",
		"  · 04  run tests",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("rendered frame missing %q:\n%s", want, joined)
		}
	}
}

func TestRenderStatusFrame(t *testing.T) {
	rows := RenderStatusFrame(2, 74*time.Second, "codencer", []StatusLine{
		{Label: "task", Value: "CLI task - Codex README check"},
		{Label: "executor", Value: "codex-workspace"},
		{Label: "state", Value: "waiting for executor result"},
	}, false)
	joined := strings.Join(rows, "\n")
	for _, want := range []string{
		"codencer   ⠹ running",
		"task: CLI task - Codex README check",
		"executor: codex-workspace",
		"state: waiting for executor result",
		"elapsed: 01:14",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("rendered status frame missing %q:\n%s", want, joined)
		}
	}
	for _, forbidden := range []string{"✓ 01", "· 01", "prepare task", "collect report"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("status frame rendered checklist content %q:\n%s", forbidden, joined)
		}
	}
}

func TestFormatElapsed(t *testing.T) {
	tests := map[time.Duration]string{
		-1 * time.Second: "00:00",
		0:                "00:00",
		74 * time.Second: "01:14",
		(2*time.Hour + 3*time.Minute + 4*time.Second): "02:03:04",
	}
	for input, want := range tests {
		if got := FormatElapsed(input); got != want {
			t.Fatalf("FormatElapsed(%s) = %q want %q", input, got, want)
		}
	}
}

func TestWorkingIndicatorTTYUsesCursorControls(t *testing.T) {
	var out bytes.Buffer
	indicator := NewWorkingIndicator(Options{
		ForceInteractive: true,
		Output:           &out,
		Stdout:           &out,
		Stderr:           &out,
	}, []string{"read schema"}, "codencer")
	indicator.Start()
	indicator.Stop(true)
	got := out.String()
	for _, want := range []string{ansiHideCursor, "\x1b[3A" + ansiClearToEnd, ansiShowCursor, "✓", "done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("TTY output missing %q: %q", want, got)
		}
	}
}

func TestWorkingIndicatorFallbackAndSilentDisabled(t *testing.T) {
	var out bytes.Buffer
	indicator := NewWorkingIndicator(Options{Output: &out}, []string{"read schema", "verify"}, "codencer")
	indicator.Start()
	got := out.String()
	for _, want := range []string{TerminalMarkCompact + " codencer", "  - 01  read schema", "  - 02  verify"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fallback missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("fallback should not contain ANSI controls: %q", got)
	}

	out.Reset()
	indicator = NewWorkingIndicator(Options{Output: &out, SilentWhenDisabled: true}, []string{"read schema"}, "codencer")
	indicator.Start()
	if out.String() != "" {
		t.Fatalf("silent disabled indicator wrote output: %q", out.String())
	}
}
