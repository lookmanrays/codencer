package cliui

import (
	"fmt"
	"io"
	"os"
)

type Options struct {
	CI          bool
	JSON        bool
	NoAnimation bool
	NoColor     bool
	Stderr      io.Writer
	Stdout      io.Writer
}

func IsInteractive(opts Options) bool {
	if opts.JSON || opts.CI || opts.NoAnimation || opts.NoColor {
		return false
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

func PrintCompactBrand(w io.Writer) {
	if w == nil {
		return
	}
	fmt.Fprintln(w, "█ ▌ ▊ ▎ codencer")
}

type Spinner struct {
	active bool
	w      io.Writer
}

func NewSpinner(opts Options) *Spinner {
	if !IsInteractive(opts) {
		return &Spinner{}
	}
	return &Spinner{active: true, w: opts.Stderr}
}

func (s *Spinner) Start(message string) {
	if !s.active {
		return
	}
	PrintCompactBrand(s.w)
	fmt.Fprintf(s.w, "* %s\n", message)
}

func (s *Spinner) Update(message string) {
	if !s.active {
		return
	}
	fmt.Fprintf(s.w, "* %s\n", message)
}

func (s *Spinner) Stop() {
	s.active = false
}

func (s *Spinner) Success(message string) {
	if !s.active {
		return
	}
	fmt.Fprintf(s.w, "ok: %s\n", message)
	s.Stop()
}

func (s *Spinner) Fail(message string) {
	if !s.active {
		return
	}
	fmt.Fprintf(s.w, "error: %s\n", message)
	s.Stop()
}
