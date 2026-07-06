package live

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
)

const (
	StatusPassed        = "passed"
	StatusFailed        = "failed"
	StatusBlocked       = "blocked"
	StatusSkipped       = "skipped"
	StatusUnsupported   = "unsupported"
	StatusNotConfigured = "not_configured"

	BlockerExecutorMissing      = "executor_missing"
	BlockerAuthRequired         = "auth_required"
	BlockerRateLimit            = "rate_limit"
	BlockerDaemonNotRunning     = "daemon_not_running"
	BlockerConnectorOffline     = "connector_offline"
	BlockerRelayUnreachable     = "relay_unreachable"
	BlockerMCPConfigInvalid     = "mcp_config_invalid"
	BlockerWSLUnavailable       = "wsl_unavailable"
	BlockerWSLSystemdDisabled   = "wsl_systemd_disabled"
	BlockerServiceRestartFailed = "service_restart_failed"
	BlockerValidationFailed     = "validation_failed"
	BlockerTimeout              = "timeout"
	BlockerManualProofRequired  = "manual_client_proof_required"
	BlockerUnknown              = "unknown"
)

const (
	EnvLiveAll            = "CODENCER_LIVE_ALL"
	EnvLiveCodex          = "CODENCER_LIVE_CODEX"
	EnvLiveClaude         = "CODENCER_LIVE_CLAUDE"
	EnvLiveRelayMCP       = "CODENCER_LIVE_RELAY_MCP"
	EnvLiveCodexMCP       = "CODENCER_LIVE_CODEX_MCP"
	EnvLiveClaudeMCP      = "CODENCER_LIVE_CLAUDE_MCP"
	EnvLiveWSL            = "CODENCER_LIVE_WSL"
	EnvLiveServiceRestart = "CODENCER_LIVE_SERVICE_RESTART"
	EnvLiveInstalledSvcs  = "CODENCER_LIVE_INSTALLED_SERVICES"
	EnvKeepWorkspace      = "CODENCER_KEEP_LIVE_WORKSPACE"
)

type Options struct {
	Profile              string
	EnableCodex          bool
	EnableClaude         bool
	EnableRelayMCP       bool
	EnableCodexMCP       bool
	EnableClaudeMCP      bool
	EnableWSL            bool
	EnableServiceRestart bool
	EnableAll            bool
	CodencerHome         string
	RepoRoot             string
	BinDir               string
	Endpoint             string
	Now                  func() time.Time
}

type Environment struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	WSL          bool   `json:"wsl"`
	CodencerHome string `json:"codencer_home"`
	Repo         string `json:"repo,omitempty"`
}

type Blocker struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

type Check struct {
	ID            string   `json:"id"`
	Category      string   `json:"category"`
	Status        string   `json:"status"`
	Live          bool     `json:"live"`
	DurationMS    int64    `json:"duration_ms"`
	ObservedFacts []string `json:"observed_facts,omitempty"`
	Blocker       *Blocker `json:"blocker,omitempty"`
	Reason        string   `json:"reason,omitempty"`
}

type Summary struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Blocked int `json:"blocked"`
	Skipped int `json:"skipped"`
}

type Report struct {
	OK          bool        `json:"ok"`
	Profile     string      `json:"profile"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt time.Time   `json:"completed_at"`
	Environment Environment `json:"environment"`
	Checks      []Check     `json:"checks"`
	Summary     Summary     `json:"summary"`
	ReportPath  string      `json:"report_path,omitempty"`
	Workspace   string      `json:"workspace,omitempty"`
}

type ReportFile struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (opts Options) IsEnabled(kind string) bool {
	if opts.EnableAll || opts.EnableAllFromEnv() {
		return true
	}
	switch kind {
	case "codex":
		return opts.EnableCodex || envEnabled(EnvLiveCodex) || envEnabled("CODENCER_LIVE_CODEX_SMOKE")
	case "claude":
		return opts.EnableClaude || envEnabled(EnvLiveClaude) || envEnabled("CODENCER_LIVE_CLAUDE_SMOKE")
	case "relay-mcp":
		return opts.EnableRelayMCP || envEnabled(EnvLiveRelayMCP)
	case "codex-mcp":
		return opts.EnableCodexMCP || envEnabled(EnvLiveCodexMCP)
	case "claude-mcp":
		return opts.EnableClaudeMCP || envEnabled(EnvLiveClaudeMCP)
	case "wsl":
		return opts.EnableWSL || envEnabled(EnvLiveWSL)
	case "restart-reconnect":
		return opts.EnableServiceRestart || envEnabled(EnvLiveServiceRestart)
	default:
		return false
	}
}

func (opts Options) EnableAllFromEnv() bool {
	return envEnabled(EnvLiveAll)
}

func envEnabled(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func NewReport(profile string, opts Options) (Report, local.Paths, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Report{}, local.Paths{}, err
		}
		repo = wd
	}
	paths, err := local.ResolvePathsForHome(repo, "", opts.CodencerHome)
	if err != nil {
		return Report{}, local.Paths{}, err
	}
	now := now(opts)
	report := Report{
		OK:        true,
		Profile:   firstNonEmpty(profile, opts.Profile, "local"),
		StartedAt: now,
		Environment: Environment{
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			WSL:          local.DetectEnvironment().WSL,
			CodencerHome: paths.Home,
			Repo:         repo,
		},
	}
	return report, paths, nil
}

func (r *Report) Add(check Check) {
	check.ObservedFacts = redactList(check.ObservedFacts)
	if check.Blocker != nil {
		check.Blocker.Message = Redact(check.Blocker.Message)
	}
	check.Reason = Redact(check.Reason)
	r.Checks = append(r.Checks, check)
}

func (r *Report) Finish(opts Options) {
	if r.CompletedAt.IsZero() {
		r.CompletedAt = now(opts)
	}
	r.Summary = Summary{}
	r.OK = true
	for _, check := range r.Checks {
		switch check.Status {
		case StatusPassed:
			r.Summary.Passed++
		case StatusFailed:
			r.Summary.Failed++
			r.OK = false
		case StatusBlocked:
			r.Summary.Blocked++
			r.OK = false
		case StatusSkipped, StatusUnsupported, StatusNotConfigured:
			r.Summary.Skipped++
		default:
			r.Summary.Blocked++
			r.OK = false
		}
	}
}

func ExitCode(report Report) int {
	for _, check := range report.Checks {
		if check.Status != StatusBlocked && check.Status != StatusFailed {
			continue
		}
		if check.Blocker == nil {
			return localexec.ExitFailedTerminal
		}
		switch check.Blocker.Type {
		case BlockerAuthRequired, BlockerRateLimit, BlockerManualProofRequired:
			return localexec.ExitBlocked
		case BlockerValidationFailed:
			return localexec.ExitValidationFailed
		case BlockerDaemonNotRunning, BlockerConnectorOffline, BlockerRelayUnreachable:
			return localexec.ExitDaemonFailed
		case BlockerTimeout:
			return localexec.ExitTimeout
		case BlockerExecutorMissing, BlockerMCPConfigInvalid:
			return localexec.ExitInvalidInput
		default:
			return localexec.ExitFailedTerminal
		}
	}
	return localexec.ExitSuccess
}

func PersistReport(paths local.Paths, subdir string, report *Report) error {
	if report == nil {
		return nil
	}
	if subdir == "" {
		subdir = "live-matrix"
	}
	dir := filepath.Join(paths.ArtifactsDir, subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	ts := report.CompletedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	path := filepath.Join(dir, ts.Format("20060102T150405.000000000Z")+".json")
	report.ReportPath = path
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func PersistJSON(paths local.Paths, subdir string, value any) (string, error) {
	dir := filepath.Join(paths.ArtifactsDir, subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, time.Now().UTC().Format("20060102T150405.000000000Z")+".json")
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func ListReports(homeOverride, subdir string) ([]ReportFile, error) {
	paths, err := local.ResolvePathsForHome("", "", homeOverride)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(paths.ArtifactsDir, subdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ReportFile{}, nil
		}
		return nil, err
	}
	files := make([]ReportFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, ReportFile{
			Path:      filepath.Join(dir, entry.Name()),
			Name:      entry.Name(),
			UpdatedAt: info.ModTime().UTC(),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name > files[j].Name })
	return files, nil
}

func timedCheck(id, category string, live bool, fn func() Check) Check {
	start := time.Now()
	check := fn()
	check.ID = firstNonEmpty(check.ID, id)
	check.Category = firstNonEmpty(check.Category, category)
	check.Live = live
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

func skipped(id, category, reason string, live bool) Check {
	return Check{ID: id, Category: category, Status: StatusSkipped, Live: live, Reason: reason}
}

func blocked(id, category, blockerType, message string, live bool) Check {
	return Check{
		ID:       id,
		Category: category,
		Status:   StatusBlocked,
		Live:     live,
		Blocker:  &Blocker{Type: blockerType, Message: message},
		Reason:   message,
	}
}

func failed(id, category, blockerType, message string, live bool) Check {
	return Check{
		ID:       id,
		Category: category,
		Status:   StatusFailed,
		Live:     live,
		Blocker:  &Blocker{Type: blockerType, Message: message},
		Reason:   message,
	}
}

func passed(id, category string, live bool, facts ...string) Check {
	return Check{ID: id, Category: category, Status: StatusPassed, Live: live, ObservedFacts: facts}
}

func now(opts Options) time.Time {
	if opts.Now != nil {
		return opts.Now().UTC()
	}
	return time.Now().UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func Redact(value string) string {
	replacements := []string{
		"Authorization: Bearer ",
		"authorization: bearer ",
		"Bearer ",
		"bearer ",
		"planner_token",
		"private_key",
		"token",
		"secret",
	}
	out := value
	for _, marker := range replacements[:4] {
		idx := strings.Index(out, marker)
		if idx < 0 {
			continue
		}
		start := idx + len(marker)
		end := start
		for end < len(out) && out[end] != ' ' && out[end] != '\n' && out[end] != '\t' && out[end] != '"' && out[end] != '\'' {
			end++
		}
		out = out[:start] + "<redacted>" + out[end:]
	}
	lower := strings.ToLower(out)
	for _, word := range replacements[4:] {
		if strings.Contains(lower, word) {
			out = strings.ReplaceAll(out, word, word[:1]+"<redacted>")
			out = strings.ReplaceAll(out, strings.ToUpper(word), strings.ToUpper(word[:1])+"<redacted>")
		}
	}
	return out
}

func redactList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, Redact(value))
	}
	return out
}

func ErrorReport(message string, code int) map[string]any {
	return map[string]any{
		"ok":        false,
		"status":    "error",
		"error":     fmt.Sprint(message),
		"exit_code": code,
	}
}
