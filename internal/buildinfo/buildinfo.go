package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	Version = "v0.3.0-local-prod-rc.1"
	Commit  = ""
	Date    = ""
	Dirty   = ""
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	Date      string `json:"date,omitempty"`
	Dirty     bool   `json:"dirty"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

func Current() Info {
	info := Info{
		Version:   firstNonEmpty(Version, "dev"),
		Commit:    strings.TrimSpace(Commit),
		Date:      strings.TrimSpace(Date),
		Dirty:     parseDirty(Dirty),
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = setting.Value
				}
			case "vcs.modified":
				if Dirty == "" {
					info.Dirty = setting.Value == "true"
				}
			}
		}
	}
	return info
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseDirty(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "dirty":
		return true
	default:
		return false
	}
}
