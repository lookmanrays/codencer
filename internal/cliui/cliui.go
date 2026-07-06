package cliui

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	Tick                 = 135 * time.Millisecond
	DefaultStepDuration  = 900 * time.Millisecond
	TerminalMarkCompact  = "█▌▊▎"
	TerminalMarkSpacious = "█ ▌ ▊ ▎"

	ansiOrange     = "\x1b[38;2;255;90;31m"
	ansiFGReset    = "\x1b[39m"
	ansiDim        = "\x1b[2m"
	ansiDimReset   = "\x1b[22m"
	ansiHideCursor = "\x1b[?25l"
	ansiShowCursor = "\x1b[?25h"
	ansiClearToEnd = "\x1b[0J"
)

var (
	markGlyphs    = []string{"█", "▌", "▊", "▎"}
	spinnerGlyphs = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

type Options struct {
	CI                 bool
	JSON               bool
	NoAnimation        bool
	NoColor            bool
	Stderr             io.Writer
	Stdout             io.Writer
	Output             io.Writer
	SilentWhenDisabled bool

	// ForceInteractive is for deterministic renderer tests. Safety guards such
	// as JSON, CI, NO_COLOR, and CODENCER_NO_ANIMATION still take precedence.
	ForceInteractive bool
}

type RenderOptions struct {
	Label  string
	Color  bool
	StepMs time.Duration
}

type WorkingIndicator struct {
	opts  Options
	steps []string
	label string
	out   io.Writer

	mu          sync.Mutex
	started     bool
	running     bool
	lines       int
	startedAt   time.Time
	stopCh      chan struct{}
	stoppedCh   chan struct{}
	cleanupOnce sync.Once
}

func IsInteractive(opts Options) bool {
	if opts.JSON || opts.CI || opts.NoAnimation || opts.NoColor {
		return false
	}
	if opts.ForceInteractive {
		return true
	}
	return isTTY(opts.Stdout) && isTTY(opts.Stderr)
}

func isTTY(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func EnvOptions(asJSON bool, stdout, stderr io.Writer) Options {
	return Options{
		CI:          os.Getenv("CI") == "true",
		JSON:        asJSON,
		NoAnimation: os.Getenv("CODENCER_NO_ANIMATION") == "1",
		NoColor:     os.Getenv("NO_COLOR") != "",
		Stderr:      stderr,
		Stdout:      stdout,
	}
}

func NewWorkingIndicator(opts Options, steps []string, label string) *WorkingIndicator {
	if label == "" {
		label = "codencer"
	}
	out := opts.Output
	if out == nil {
		out = opts.Stdout
	}
	return &WorkingIndicator{
		opts:  opts,
		steps: append([]string(nil), steps...),
		label: label,
		out:   out,
	}
}

func (w *WorkingIndicator) Start() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return
	}
	w.started = true
	if w.out == nil {
		return
	}
	if !IsInteractive(w.opts) {
		if !w.opts.SilentWhenDisabled {
			writeFallback(w.out, w.label, w.steps)
		}
		return
	}
	w.running = true
	w.startedAt = time.Now()
	w.stopCh = make(chan struct{})
	w.stoppedCh = make(chan struct{})
	fmt.Fprint(w.out, ansiHideCursor)
	w.redrawLocked(RenderWorkingFrame(0, 0, w.steps, RenderOptions{
		Label:  w.label,
		Color:  true,
		StepMs: DefaultStepDuration,
	}))
	go w.loop(w.stopCh, w.stoppedCh)
}

func (w *WorkingIndicator) Stop(ok bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	stopCh := w.stopCh
	stoppedCh := w.stoppedCh
	w.running = false
	w.mu.Unlock()

	w.cleanupOnce.Do(func() {
		close(stopCh)
		<-stoppedCh
	})

	w.mu.Lock()
	defer w.mu.Unlock()
	w.redrawLocked([]string{FinalLine(w.label, ok, true)})
	fmt.Fprint(w.out, ansiShowCursor)
}

func (w *WorkingIndicator) loop(stopCh <-chan struct{}, stoppedCh chan<- struct{}) {
	ticker := time.NewTicker(Tick)
	defer func() {
		ticker.Stop()
		_ = recover()
		close(stoppedCh)
	}()
	for {
		select {
		case <-ticker.C:
			w.render()
		case <-stopCh:
			return
		}
	}
}

func (w *WorkingIndicator) render() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	elapsed := time.Since(w.startedAt)
	bucket := int(elapsed / Tick)
	w.redrawLocked(RenderWorkingFrame(bucket, elapsed, w.steps, RenderOptions{
		Label:  w.label,
		Color:  true,
		StepMs: DefaultStepDuration,
	}))
}

func (w *WorkingIndicator) redrawLocked(rows []string) {
	if w.out == nil {
		return
	}
	if w.lines > 0 {
		fmt.Fprintf(w.out, "\x1b[%dA%s", w.lines, ansiClearToEnd)
	}
	fmt.Fprint(w.out, strings.Join(rows, "\n"))
	fmt.Fprint(w.out, "\n")
	w.lines = len(rows)
}

func MarkFrame(bucket int, live bool, color bool) string {
	lit := [4]bool{}
	if live {
		any := false
		for i := range lit {
			if deterministicRand(bucket, i) > 0.6 {
				lit[i] = true
				any = true
			}
		}
		if !any {
			lit[positiveMod(bucket, len(lit))] = true
		}
	} else {
		lit[0] = true
		lit[2] = true
	}

	var b strings.Builder
	for i, glyph := range markGlyphs {
		if lit[i] {
			b.WriteString(colorize(glyph, color))
			continue
		}
		b.WriteString(glyph)
	}
	return b.String()
}

func SpinnerFrame(bucket int) string {
	return spinnerGlyphs[positiveMod(bucket, len(spinnerGlyphs))]
}

func RenderWorkingFrame(bucket int, elapsed time.Duration, steps []string, opts RenderOptions) []string {
	label := opts.Label
	if label == "" {
		label = "codencer"
	}
	stepMs := opts.StepMs
	if stepMs <= 0 {
		stepMs = DefaultStepDuration
	}
	spinner := SpinnerFrame(bucket)
	rows := []string{fmt.Sprintf("%s %s   %s %s", MarkFrame(bucket, true, opts.Color), label, colorize(spinner, opts.Color), dim("running", opts.Color))}
	if len(steps) == 0 {
		return rows
	}
	active := int(elapsed / stepMs)
	if active > len(steps) {
		active = len(steps)
	}
	rows = append(rows, "")
	for i, step := range steps {
		n := pad2(i)
		switch {
		case i < active:
			rows = append(rows, fmt.Sprintf("  %s %s  %s", dim("✓", opts.Color), dim(n, opts.Color), step))
		case i == active:
			rows = append(rows, fmt.Sprintf("  %s %s  %s", colorize(spinner, opts.Color), n, step))
		default:
			rows = append(rows, dim(fmt.Sprintf("  · %s  %s", n, step), opts.Color))
		}
	}
	return rows
}

func FinalLine(label string, ok bool, color bool) string {
	if label == "" {
		label = "codencer"
	}
	if ok {
		return fmt.Sprintf("%s %s   %s %s", MarkFrame(0, false, color), label, colorize("✓", color), dim("done", color))
	}
	return fmt.Sprintf("%s %s   %s %s", MarkFrame(0, false, color), label, dim("·", color), dim("idle", color))
}

func WriteFallback(w io.Writer, label string, steps []string) {
	writeFallback(w, label, steps)
}

func writeFallback(w io.Writer, label string, steps []string) {
	if w == nil {
		return
	}
	if label == "" {
		label = "codencer"
	}
	fmt.Fprintf(w, "%s %s\n", MarkFrame(0, false, false), label)
	for i, step := range steps {
		fmt.Fprintf(w, "  - %s  %s\n", pad2(i), step)
	}
}

func colorize(s string, color bool) string {
	if !color {
		return s
	}
	return ansiOrange + s + ansiFGReset
}

func dim(s string, color bool) string {
	if !color {
		return s
	}
	return ansiDim + s + ansiDimReset
}

func deterministicRand(bucket int, i int) float64 {
	h := math.Sin(float64(bucket)*12.9898+float64(i)*78.233) * 43758.5453
	return h - math.Floor(h)
}

func positiveMod(value int, mod int) int {
	if mod <= 0 {
		return 0
	}
	out := value % mod
	if out < 0 {
		out += mod
	}
	return out
}

func pad2(i int) string {
	return fmt.Sprintf("%02d", i+1)
}
