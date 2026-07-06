package app

import "agent-bridge/internal/buildinfo"

// Version represents the current version of the application.
// This should be injected at build time using ldflags, e.g.:
// go build -ldflags "-X agent-bridge/internal/app.Version=v0.2.0-beta"
var Version = buildinfo.Version
