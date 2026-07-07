package release

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	DefaultTargetSpec    = "darwin/arm64,darwin/amd64,linux/amd64"
	DefaultDockerImage   = "golang:1.25-bookworm"
	artifactStatusBuilt  = "built"
	artifactStatusFailed = "not_built"
	artifactStatusSkip   = "skipped"
)

type CommandRunner func(*exec.Cmd) ([]byte, error)

type Options struct {
	Version        string
	RepoRoot       string
	DistDir        string
	DryRun         bool
	Targets        string
	RequireTargets string
	AllowPartial   bool
	DockerImage    string
	Now            func() time.Time
	CommandRunner  CommandRunner
}

type Artifact struct {
	Name     string `json:"name"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	SHA256   string `json:"sha256,omitempty"`
	Status   string `json:"status"`
	Required bool   `json:"required"`
	Mode     string `json:"mode,omitempty"`
	Message  string `json:"message,omitempty"`
}

type Manifest struct {
	Version         string     `json:"version"`
	Commit          string     `json:"commit"`
	BuiltAt         time.Time  `json:"built_at"`
	Targets         []string   `json:"targets"`
	RequiredTargets []string   `json:"required_targets"`
	AllowPartial    bool       `json:"allow_partial"`
	Partial         bool       `json:"partial"`
	Artifacts       []Artifact `json:"artifacts"`
}

type Report struct {
	OK              bool       `json:"ok"`
	Partial         bool       `json:"partial"`
	DistDir         string     `json:"dist_dir"`
	ManifestPath    string     `json:"manifest_path"`
	ChecksumsPath   string     `json:"checksums_path"`
	Targets         []string   `json:"targets"`
	RequiredTargets []string   `json:"required_targets"`
	Artifacts       []Artifact `json:"artifacts"`
	Errors          []string   `json:"errors,omitempty"`
}

type target struct {
	OS   string
	Arch string
}

type binarySpec struct {
	name string
	pkg  string
}

var releaseBinaries = []binarySpec{
	{name: "codencer", pkg: "./cmd/codencer"},
	{name: "orchestratord", pkg: "./cmd/orchestratord"},
	{name: "codencer-relayd", pkg: "./cmd/codencer-relayd"},
	{name: "codencer-gatewayd", pkg: "./cmd/codencer-gatewayd"},
	{name: "codencer-connectord", pkg: "./cmd/codencer-connectord"},
}

func Snapshot(opts Options) (Report, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		var err error
		repo, err = os.Getwd()
		if err != nil {
			return Report{}, err
		}
	}
	repo, err := filepath.Abs(repo)
	if err != nil {
		return Report{}, err
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		return Report{}, fmt.Errorf("version is required")
	}
	dist := strings.TrimSpace(opts.DistDir)
	if dist == "" {
		dist = filepath.Join(repo, "dist")
	}
	if !filepath.IsAbs(dist) {
		dist = filepath.Join(repo, dist)
	}
	targets, err := parseTargets(firstNonEmpty(opts.Targets, DefaultTargetSpec))
	if err != nil {
		return Report{}, err
	}
	requiredTargets, err := parseRequiredTargets(opts.RequireTargets, targets)
	if err != nil {
		return Report{}, err
	}
	required := targetSet(requiredTargets)
	if err := os.MkdirAll(dist, 0755); err != nil {
		return Report{}, err
	}
	work := filepath.Join(dist, ".work-"+sanitize(version))
	_ = os.RemoveAll(work)
	if err := os.MkdirAll(work, 0755); err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(work)

	manifest := Manifest{
		Version:         version,
		Commit:          gitCommit(repo),
		BuiltAt:         now(opts.Now),
		Targets:         targetStrings(targets),
		RequiredTargets: targetStrings(requiredTargets),
		AllowPartial:    opts.AllowPartial,
	}
	checksumLines := []string{}
	runner := opts.CommandRunner
	if runner == nil {
		runner = func(cmd *exec.Cmd) ([]byte, error) { return cmd.CombinedOutput() }
	}
	dockerImage := firstNonEmpty(opts.DockerImage, DefaultDockerImage)
	for _, tgt := range targets {
		artifact := buildTarget(repo, work, dist, version, manifest.Commit, tgt, required[tgt.String()], opts.DryRun, dockerImage, runner)
		manifest.Artifacts = append(manifest.Artifacts, artifact)
		if artifact.Status == artifactStatusBuilt {
			checksumLines = append(checksumLines, artifact.SHA256+"  "+artifact.Name)
		}
	}
	manifest.Partial = isPartial(manifest.Artifacts)
	manifestPath := filepath.Join(dist, "manifest.json")
	checksumsPath := filepath.Join(dist, "checksums.txt")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return Report{}, err
	}
	if err := os.WriteFile(checksumsPath, []byte(strings.Join(checksumLines, "\n")+"\n"), 0644); err != nil {
		return Report{}, err
	}

	report := Report{
		Partial:         manifest.Partial,
		DistDir:         dist,
		ManifestPath:    manifestPath,
		ChecksumsPath:   checksumsPath,
		Targets:         manifest.Targets,
		RequiredTargets: manifest.RequiredTargets,
		Artifacts:       manifest.Artifacts,
	}
	if !opts.DryRun {
		report.Errors = append(report.Errors, requiredTargetErrors(manifest.Artifacts)...)
	}
	if err := ValidateDist(dist); err != nil {
		report.Errors = append(report.Errors, err.Error())
	}
	report.OK = reportOK(opts.DryRun, opts.AllowPartial, manifest.Artifacts, report.Errors)
	return report, nil
}

func ValidateDist(dist string) error {
	manifestPath := filepath.Join(dist, "manifest.json")
	checksumsPath := filepath.Join(dist, "checksums.txt")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("release manifest missing: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("release manifest invalid: %w", err)
	}
	expected, err := readChecksums(checksumsPath)
	if err != nil {
		return err
	}
	var problems []string
	for _, artifact := range manifest.Artifacts {
		if artifact.Status != artifactStatusBuilt {
			continue
		}
		if strings.TrimSpace(artifact.SHA256) == "" {
			problems = append(problems, artifact.Name+": built artifact missing sha256")
			continue
		}
		want, ok := expected[artifact.Name]
		if !ok {
			problems = append(problems, artifact.Name+": missing from checksums.txt")
			continue
		}
		if want != artifact.SHA256 {
			problems = append(problems, artifact.Name+": checksum manifest/checksums mismatch")
			continue
		}
		path := filepath.Join(dist, artifact.Name)
		got, err := sha256File(path)
		if err != nil {
			problems = append(problems, artifact.Name+": built artifact missing on disk")
			continue
		}
		if got != want {
			problems = append(problems, artifact.Name+": checksum does not match file")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("release artifact validation failed: %s", strings.Join(problems, "; "))
	}
	return nil
}

func buildTarget(repo, work, dist, version, commit string, tgt target, required, dryRun bool, dockerImage string, runner CommandRunner) Artifact {
	ext := ""
	if tgt.OS == "windows" {
		ext = ".exe"
	}
	name := fmt.Sprintf("codencer_%s_%s_%s", version, tgt.OS, tgt.Arch)
	archiveName := name + ".tar.gz"
	if tgt.OS == "windows" {
		archiveName = name + ".zip"
	}
	mode := buildMode(tgt)
	artifact := Artifact{Name: archiveName, OS: tgt.OS, Arch: tgt.Arch, Status: artifactStatusFailed, Required: required, Mode: mode}
	if dryRun {
		artifact.Status = artifactStatusSkip
		artifact.Message = "dry run"
		return artifact
	}
	stage := filepath.Join(work, name)
	binDir := filepath.Join(stage, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		artifact.Message = err.Error()
		return artifact
	}
	ldflags := fmt.Sprintf("-X agent-bridge/internal/app.Version=%s -X agent-bridge/internal/buildinfo.Version=%s -X agent-bridge/internal/buildinfo.Commit=%s -X agent-bridge/internal/buildinfo.Date=%s", version, version, commit, time.Now().UTC().Format(time.RFC3339))
	for _, binary := range releaseBinaries {
		out := filepath.Join(binDir, binary.name+ext)
		cmd, err := buildCommand(repo, out, binary.pkg, ldflags, tgt, mode, dockerImage)
		if err != nil {
			artifact.Message = err.Error()
			return artifact
		}
		if output, err := runner(cmd); err != nil {
			artifact.Message = fmt.Sprintf("%s build failed: %v: %s", binary.name, err, strings.TrimSpace(string(output)))
			return artifact
		}
	}
	_ = buildBroker(repo, binDir, tgt, ext, mode, dockerImage, runner)
	if err := writeBundleFiles(repo, stage); err != nil {
		artifact.Message = err.Error()
		return artifact
	}
	archivePath := filepath.Join(dist, archiveName)
	var err error
	if tgt.OS == "windows" {
		err = zipDir(stage, archivePath)
	} else {
		err = tarGzDir(stage, archivePath)
	}
	if err != nil {
		artifact.Message = err.Error()
		return artifact
	}
	sum, err := sha256File(archivePath)
	if err != nil {
		artifact.Message = err.Error()
		return artifact
	}
	artifact.SHA256 = sum
	artifact.Status = artifactStatusBuilt
	if mode == "docker" {
		artifact.Message = "built in Docker Linux environment; not product-live verified"
	} else if tgt.OS != runtime.GOOS || tgt.Arch != runtime.GOARCH {
		artifact.Message = "cross-built on current host; not notarized or product-live verified"
	}
	return artifact
}

func buildCommand(repo, out, pkg, ldflags string, tgt target, mode, dockerImage string) (*exec.Cmd, error) {
	if mode == "docker" {
		relOut, err := filepath.Rel(repo, out)
		if err != nil {
			return nil, err
		}
		containerOut := path.Join("/workspace", filepath.ToSlash(relOut))
		return exec.Command("docker", "run", "--rm",
			"--platform", tgt.OS+"/"+tgt.Arch,
			"-v", repo+":/workspace",
			"-w", "/workspace",
			"-e", "GOOS="+tgt.OS,
			"-e", "GOARCH="+tgt.Arch,
			"-e", "CGO_ENABLED=1",
			dockerImage,
			"go", "build", "-ldflags", ldflags, "-o", containerOut, pkg,
		), nil
	}
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", out, pkg)
	cmd.Dir = repo
	cmd.Env = targetEnv(tgt)
	return cmd, nil
}

func targetEnv(tgt target) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GOOS="+tgt.OS, "GOARCH="+tgt.Arch, "CGO_ENABLED=1")
	return env
}

func buildMode(tgt target) string {
	if tgt.OS == "linux" && runtime.GOOS != "linux" {
		return "docker"
	}
	return "host"
}

func buildBroker(repo, binDir string, tgt target, ext, mode, dockerImage string, runner CommandRunner) error {
	brokerDir := filepath.Join(repo, "cmd", "broker")
	if _, err := os.Stat(filepath.Join(brokerDir, "go.mod")); err != nil {
		return err
	}
	out := filepath.Join(binDir, "agent-broker"+ext)
	var cmd *exec.Cmd
	var err error
	if mode == "docker" {
		relOut, err := filepath.Rel(repo, out)
		if err != nil {
			return err
		}
		containerOut := path.Join("/workspace", filepath.ToSlash(relOut))
		cmd = exec.Command("docker", "run", "--rm",
			"--platform", tgt.OS+"/"+tgt.Arch,
			"-v", repo+":/workspace",
			"-w", "/workspace/cmd/broker",
			"-e", "GOOS="+tgt.OS,
			"-e", "GOARCH="+tgt.Arch,
			"-e", "CGO_ENABLED=1",
			dockerImage,
			"go", "build", "-o", containerOut, ".",
		)
	} else {
		cmd = exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = brokerDir
		cmd.Env = targetEnv(tgt)
	}
	_, err = runner(cmd)
	return err
}

func writeBundleFiles(repo, stage string) error {
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	quickstart := "Codencer local production release snapshot\n\n" +
		"Run ./scripts/install.sh --dry-run first. Use --bin-dir ./bin only when explicitly overriding the package-local binary directory.\n\n" +
		"Local setup: codencer setup local --json\n" +
		"Self-host setup: codencer setup self-host --gateway-url http://127.0.0.1:19090 --relay-url http://127.0.0.1:8090 --relay-request-timeout-seconds 300 --token-env CODENCER_GATEWAY_MCP_TOKEN --enable-oauth-dev --json\n" +
		"Gateway activation: codencer activation self-host --gateway http://127.0.0.1:19090 --relay http://127.0.0.1:8090 --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json\n"
	if err := os.WriteFile(filepath.Join(stage, "QUICKSTART.txt"), []byte(quickstart), 0644); err != nil {
		return err
	}
	for _, rel := range []string{
		"README.md",
		"LICENSE",
		"NOTICE",
		"TRADEMARKS.md",
		"SECURITY.md",
		"CONTRIBUTING.md",
		"CODE_OF_CONDUCT.md",
	} {
		if err := copyFile(filepath.Join(repo, rel), filepath.Join(stage, rel)); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
	}
	for _, rel := range []string{"scripts/install.sh", "scripts/uninstall.sh", "scripts/upgrade.sh"} {
		if err := copyFile(filepath.Join(repo, rel), filepath.Join(stage, rel)); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
	}
	if err := copyTree(filepath.Join(repo, "docs"), filepath.Join(stage, "docs")); err != nil {
		return fmt.Errorf("copy docs: %w", err)
	}
	return nil
}

func tarGzDir(src, dst string) error {
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(src), path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

func zipDir(src, dst string) error {
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	defer zw.Close()
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == src || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(src), path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, info.Mode().Perm())
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func parseTargets(spec string) ([]target, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = DefaultTargetSpec
	}
	if spec == "host" {
		return []target{{OS: runtime.GOOS, Arch: runtime.GOARCH}}, nil
	}
	parts := strings.Split(spec, ",")
	targets := make([]target, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		tgt, err := parseTarget(part)
		if err != nil {
			return nil, err
		}
		if seen[tgt.String()] {
			continue
		}
		seen[tgt.String()] = true
		targets = append(targets, tgt)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one release target is required")
	}
	return targets, nil
}

func parseRequiredTargets(spec string, selected []target) ([]target, error) {
	if strings.TrimSpace(spec) == "" {
		return append([]target{}, selected...), nil
	}
	required, err := parseTargets(spec)
	if err != nil {
		return nil, err
	}
	selectedSet := targetSet(selected)
	for _, tgt := range required {
		if !selectedSet[tgt.String()] {
			return nil, fmt.Errorf("required target %s is not included in selected targets", tgt.String())
		}
	}
	return required, nil
}

func parseTarget(value string) (target, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return target{}, fmt.Errorf("empty release target")
	}
	if value == "host" {
		return target{OS: runtime.GOOS, Arch: runtime.GOARCH}, nil
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return target{}, fmt.Errorf("invalid release target %q: expected os/arch", value)
	}
	if parts[0] == "windows" {
		return target{}, fmt.Errorf("windows native release target is not claimed in Sprint 6.1; use WSL2/linux")
	}
	return target{OS: parts[0], Arch: parts[1]}, nil
}

func (t target) String() string {
	return t.OS + "/" + t.Arch
}

func targetSet(targets []target) map[string]bool {
	out := map[string]bool{}
	for _, tgt := range targets {
		out[tgt.String()] = true
	}
	return out
}

func targetStrings(targets []target) []string {
	values := make([]string, 0, len(targets))
	for _, tgt := range targets {
		values = append(values, tgt.String())
	}
	sort.Strings(values)
	return values
}

func isPartial(artifacts []Artifact) bool {
	for _, artifact := range artifacts {
		if artifact.Status != artifactStatusBuilt {
			return true
		}
	}
	return false
}

func requiredTargetErrors(artifacts []Artifact) []string {
	var errors []string
	for _, artifact := range artifacts {
		if artifact.Required && artifact.Status != artifactStatusBuilt {
			errors = append(errors, fmt.Sprintf("required target %s/%s was %s: %s", artifact.OS, artifact.Arch, artifact.Status, artifact.Message))
		}
	}
	return errors
}

func reportOK(dryRun, allowPartial bool, artifacts []Artifact, errors []string) bool {
	if dryRun {
		return len(errors) == 0
	}
	if len(errors) == 0 {
		return true
	}
	for _, err := range errors {
		if !strings.HasPrefix(err, "required target ") {
			return false
		}
	}
	if !allowPartial {
		return false
	}
	for _, artifact := range artifacts {
		if artifact.Status == artifactStatusBuilt {
			return true
		}
	}
	return false
}

func readChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("checksums.txt missing: %w", err)
	}
	defer file.Close()
	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line %q", line)
		}
		out[fields[1]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func gitCommit(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func sanitize(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-")
	return replacer.Replace(value)
}

func now(fn func() time.Time) time.Time {
	if fn != nil {
		return fn().UTC()
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
