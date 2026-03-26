package app

// Version represents the current version of the application.
// This should be injected at build time using ldflags, e.g.:
// go build -ldflags "-X agent-bridge/internal/app.Version=v0.0.1"
var Version = "v0.1.0"
