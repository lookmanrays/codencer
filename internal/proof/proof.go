package proof

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/buildinfo"
	"agent-bridge/internal/live"
	"agent-bridge/internal/local"
	"agent-bridge/internal/project"
	"agent-bridge/internal/security"
)

type Options struct {
	CodencerHome string
	RepoRoot     string
	Now          func() time.Time
}

type ProjectSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	RepoLabel       string `json:"repo_label,omitempty"`
	RepoRootHash    string `json:"repo_root_hash,omitempty"`
	DefaultAdapter  string `json:"default_adapter"`
	Profile         string `json:"profile"`
	SharedToRelay   bool   `json:"shared_to_relay"`
	RelayInstanceID string `json:"relay_instance_id,omitempty"`
}

type BundleReport struct {
	OK              bool              `json:"ok"`
	BundleDir       string            `json:"bundle_dir"`
	ProofPath       string            `json:"proof_path"`
	ReadmePath      string            `json:"readme_path"`
	ReportsDir      string            `json:"reports_dir"`
	Build           buildinfo.Info    `json:"build"`
	Paths           map[string]string `json:"paths"`
	Projects        []ProjectSummary  `json:"projects"`
	IncludedReports []string          `json:"included_reports"`
	ReferencedFiles []string          `json:"referenced_files,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

func Bundle(opts Options) (BundleReport, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		var err error
		repo, err = os.Getwd()
		if err != nil {
			return BundleReport{}, err
		}
	}
	paths, err := local.ResolvePathsForHome(repo, "", opts.CodencerHome)
	if err != nil {
		return BundleReport{}, err
	}
	if _, err := local.EnsureHome(paths, now(opts.Now)); err != nil {
		return BundleReport{}, err
	}
	ts := now(opts.Now)
	bundleDir := filepath.Join(paths.ArtifactsDir, "proof-bundles", ts.Format("20060102T150405.000000000Z"))
	reportsDir := filepath.Join(bundleDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return BundleReport{}, err
	}
	registry, _ := project.LoadRegistry(paths.ProjectsFile)
	report := BundleReport{
		OK:         true,
		BundleDir:  bundleDir,
		ProofPath:  filepath.Join(bundleDir, "proof.json"),
		ReadmePath: filepath.Join(bundleDir, "README.md"),
		ReportsDir: reportsDir,
		Build:      buildinfo.Current(),
		Paths: map[string]string{
			"codencer_home": paths.Home,
			"config_file":   paths.ConfigFile,
			"projects_file": paths.ProjectsFile,
			"logs_dir":      paths.LogsDir,
			"artifacts_dir": paths.ArtifactsDir,
		},
		Projects:  safeProjects(registry),
		CreatedAt: ts,
	}
	for _, subdir := range []string{"readiness", "live-matrix", "acceptance"} {
		files, err := live.ListReports(opts.CodencerHome, subdir)
		if err != nil || len(files) == 0 {
			continue
		}
		src := files[0].Path
		dst := filepath.Join(reportsDir, subdir+"-"+filepath.Base(src))
		if err := copyJSONReport(src, dst); err == nil {
			report.IncludedReports = append(report.IncludedReports, dst)
		}
	}
	for _, rel := range []string{"dist/manifest.json", "dist/checksums.txt"} {
		path := filepath.Join(repo, rel)
		if _, err := os.Stat(path); err == nil {
			report.ReferencedFiles = append(report.ReferencedFiles, path)
		}
	}
	if err := writeReadme(report); err != nil {
		return report, err
	}
	if err := writeProof(report); err != nil {
		return report, err
	}
	return report, nil
}

func safeProjects(registry *project.Registry) []ProjectSummary {
	if registry == nil {
		return nil
	}
	out := make([]ProjectSummary, 0, len(registry.Projects))
	for _, p := range project.ListProjects(registry) {
		label, hash := security.SafePathLabel(p.RepoRoot)
		out = append(out, ProjectSummary{
			ID:              p.ID,
			Name:            p.Name,
			RepoLabel:       label,
			RepoRootHash:    hash,
			DefaultAdapter:  p.DefaultAdapter,
			Profile:         p.AdapterProfile,
			SharedToRelay:   p.SharedToRelay,
			RelayInstanceID: p.RelayInstanceID,
		})
	}
	return out
}

func copyJSONReport(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if !json.Valid(data) {
		return fmt.Errorf("%s is not valid JSON", src)
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	payload = security.RedactJSON(payload)
	data, err = json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(dst, data, 0600)
}

func writeReadme(report BundleReport) error {
	body := "Codencer proof bundle\n\n" +
		"This bundle contains JSON report references for local/self-host production readiness. It intentionally excludes full logs and secrets.\n\n" +
		fmt.Sprintf("Build: %s %s\n", report.Build.Version, report.Build.Commit) +
		fmt.Sprintf("Proof: %s\n", report.ProofPath)
	return os.WriteFile(report.ReadmePath, []byte(body), 0644)
}

func writeProof(report BundleReport) error {
	data, err := json.MarshalIndent(security.RedactJSON(map[string]any{
		"ok":               report.OK,
		"bundle_dir":       report.BundleDir,
		"proof_path":       report.ProofPath,
		"readme_path":      report.ReadmePath,
		"reports_dir":      report.ReportsDir,
		"build":            report.Build,
		"paths":            report.Paths,
		"projects":         report.Projects,
		"included_reports": report.IncludedReports,
		"referenced_files": report.ReferencedFiles,
		"created_at":       report.CreatedAt,
	}), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(report.ProofPath, data, 0600)
}

func now(fn func() time.Time) time.Time {
	if fn != nil {
		return fn().UTC()
	}
	return time.Now().UTC()
}
