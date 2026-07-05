package cliui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestIsInteractiveGuardsMachineReadableOutput(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "json", opts: Options{JSON: true, Stderr: os.Stderr}},
		{name: "ci", opts: Options{CI: true, Stderr: os.Stderr}},
		{name: "no animation", opts: Options{NoAnimation: true, Stderr: os.Stderr}},
		{name: "no color", opts: Options{NoColor: true, Stderr: os.Stderr}},
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

func TestSpinnerNoopsWhenNotInteractive(t *testing.T) {
	var stderr bytes.Buffer
	spinner := NewSpinner(Options{Stderr: &stderr})
	spinner.Start("configuring")
	spinner.Update("still configuring")
	spinner.Success("configured")
	if stderr.String() != "" {
		t.Fatalf("non-interactive spinner wrote output: %q", stderr.String())
	}
}

func TestPrintCompactBrand(t *testing.T) {
	var out bytes.Buffer
	PrintCompactBrand(&out)
	if got := out.String(); !strings.Contains(got, "codencer") || !strings.Contains(got, "█ ▌ ▊ ▎") {
		t.Fatalf("brand output missing expected mark/wordmark: %q", got)
	}
}
