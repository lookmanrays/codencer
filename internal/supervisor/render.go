package supervisor

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

func Render(spec ServiceSpec, format string) (string, error) {
	switch strings.TrimSpace(format) {
	case "", ManagerLaunchd:
		return RenderLaunchd(spec)
	case ManagerSystemd:
		return RenderSystemd(spec)
	default:
		return "", fmt.Errorf("unsupported render format %q", format)
	}
}

func RenderLaunchd(spec ServiceSpec) (string, error) {
	if spec.Label == "" {
		spec.Label = "io.codencer." + spec.Name
	}
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	buf.WriteString(`<plist version="1.0">` + "\n")
	buf.WriteString("<dict>\n")
	writeKeyString(&buf, "Label", spec.Label)
	writeKeyArray(&buf, "ProgramArguments", append([]string{spec.Binary}, spec.Args...))
	if spec.WorkingDir != "" {
		writeKeyString(&buf, "WorkingDirectory", spec.WorkingDir)
	}
	if len(spec.Env) > 0 {
		buf.WriteString("  <key>EnvironmentVariables</key>\n  <dict>\n")
		keys := sortedMapKeys(spec.Env)
		for _, key := range keys {
			writeKeyStringIndented(&buf, key, spec.Env[key], "    ")
		}
		buf.WriteString("  </dict>\n")
	}
	writeKeyString(&buf, "StandardOutPath", spec.StdoutLog)
	writeKeyString(&buf, "StandardErrorPath", spec.StderrLog)
	buf.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	buf.WriteString("  <key>KeepAlive</key>\n  <true/>\n")
	buf.WriteString("</dict>\n</plist>\n")
	return buf.String(), nil
}

func RenderSystemd(spec ServiceSpec) (string, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[Unit]\nDescription=Codencer %s service\n", spec.Name)
	if len(spec.Dependencies) > 0 {
		after := make([]string, 0, len(spec.Dependencies))
		for _, dep := range spec.Dependencies {
			after = append(after, "codencer-"+dep+".service")
		}
		fmt.Fprintf(&buf, "After=%s\n", strings.Join(after, " "))
	}
	buf.WriteString("\n[Service]\nType=simple\n")
	if spec.WorkingDir != "" {
		fmt.Fprintf(&buf, "WorkingDirectory=%s\n", systemdEscapeValue(spec.WorkingDir))
	}
	keys := sortedMapKeys(spec.Env)
	for _, key := range keys {
		fmt.Fprintf(&buf, "Environment=%s=%s\n", key, systemdEscapeValue(spec.Env[key]))
	}
	fmt.Fprintf(&buf, "ExecStart=%s\n", strings.Join(systemdArgs(append([]string{spec.Binary}, spec.Args...)), " "))
	if spec.StdoutLog != "" {
		fmt.Fprintf(&buf, "StandardOutput=append:%s\n", systemdEscapeValue(spec.StdoutLog))
	}
	if spec.StderrLog != "" {
		fmt.Fprintf(&buf, "StandardError=append:%s\n", systemdEscapeValue(spec.StderrLog))
	}
	buf.WriteString("Restart=on-failure\nRestartSec=5s\n")
	buf.WriteString("\n[Install]\nWantedBy=default.target\n")
	return buf.String(), nil
}

func writeKeyString(buf *bytes.Buffer, key, value string) {
	writeKeyStringIndented(buf, key, value, "  ")
}

func writeKeyStringIndented(buf *bytes.Buffer, key, value, indent string) {
	fmt.Fprintf(buf, "%s<key>%s</key>\n%s<string>%s</string>\n", indent, xmlEscape(key), indent, xmlEscape(value))
}

func writeKeyArray(buf *bytes.Buffer, key string, values []string) {
	fmt.Fprintf(buf, "  <key>%s</key>\n  <array>\n", xmlEscape(key))
	for _, value := range values {
		fmt.Fprintf(buf, "    <string>%s</string>\n", xmlEscape(value))
	}
	buf.WriteString("  </array>\n")
}

func xmlEscape(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func systemdArgs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, systemdEscapeValue(value))
	}
	return out
}

func systemdEscapeValue(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\"'\\$") {
		return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
	}
	return value
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
