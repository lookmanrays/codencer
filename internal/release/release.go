package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Options struct {
	Version  string
	RepoRoot string
	DistDir  string
	DryRun   bool
	Now      func() time.Time
}

type Artifact struct {
	Name    string `json:"name"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	SHA256  string `json:"sha256,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Manifest struct {
	Version   string     `json:"version"`
	Commit    string     `json:"commit"`
	BuiltAt   time.Time  `json:"built_at"`
	Artifacts []Artifact `json:"artifacts"`
}

type Report struct {
	OK            bool       `json:"ok"`
	DistDir       string     `json:"dist_dir"`
	ManifestPath  string     `json:"manifest_path"`
	ChecksumsPath string     `json:"checksums_path"`
	Artifacts     []Artifact `json:"artifacts"`
}

type target struct {
	OS   string
	Arch string
}

var targets = []target{
	{OS: "darwin", Arch: "arm64"},
	{OS: "darwin", Arch: "amd64"},
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "windows", Arch: "amd64"},
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
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		return Report{}, fmt.Errorf("version is required")
	}
	dist := strings.TrimSpace(opts.DistDir)
	if dist == "" {
		dist = filepath.Join(repo, "dist")
	}
	if err := os.MkdirAll(dist, 0755); err != nil {
		return Report{}, err
	}
	work := filepath.Join(dist, ".work-"+sanitize(version))
	_ = os.RemoveAll(work)
	if err := os.MkdirAll(work, 0755); err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(work)
	manifest := Manifest{Version: version, Commit: gitCommit(repo), BuiltAt: now(opts.Now)}
	checksumLines := []string{}
	for _, tgt := range targets {
		artifact := buildTarget(repo, work, dist, version, manifest.Commit, tgt, opts.DryRun)
		manifest.Artifacts = append(manifest.Artifacts, artifact)
		if artifact.Status == "built" {
			checksumLines = append(checksumLines, artifact.SHA256+"  "+artifact.Name)
		}
	}
	manifestPath := filepath.Join(dist, "manifest.json")
	checksumsPath := filepath.Join(dist, "checksums.txt")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return Report{}, err
	}
	if err := os.WriteFile(checksumsPath, []byte(strings.Join(checksumLines, "\n")+"\n"), 0644); err != nil {
		return Report{}, err
	}
	ok := opts.DryRun
	for _, artifact := range manifest.Artifacts {
		if artifact.Status == "built" {
			ok = true
			break
		}
	}
	return Report{OK: ok, DistDir: dist, ManifestPath: manifestPath, ChecksumsPath: checksumsPath, Artifacts: manifest.Artifacts}, nil
}

func buildTarget(repo, work, dist, version, commit string, tgt target, dryRun bool) Artifact {
	ext := ""
	if tgt.OS == "windows" {
		ext = ".exe"
	}
	name := fmt.Sprintf("codencer_%s_%s_%s", version, tgt.OS, tgt.Arch)
	archiveName := name + ".tar.gz"
	if tgt.OS == "windows" {
		archiveName = name + ".zip"
	}
	artifact := Artifact{Name: archiveName, OS: tgt.OS, Arch: tgt.Arch, Status: "not_built"}
	if dryRun {
		artifact.Status = "skipped"
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
	for _, binary := range []struct {
		name string
		pkg  string
	}{
		{name: "codencer", pkg: "./cmd/codencer"},
		{name: "orchestratord", pkg: "./cmd/orchestratord"},
		{name: "codencer-relayd", pkg: "./cmd/codencer-relayd"},
		{name: "codencer-connectord", pkg: "./cmd/codencer-connectord"},
	} {
		out := filepath.Join(binDir, binary.name+ext)
		cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", out, binary.pkg)
		cmd.Dir = repo
		cmd.Env = targetEnv(tgt)
		if output, err := cmd.CombinedOutput(); err != nil {
			artifact.Message = fmt.Sprintf("%s build failed: %v: %s", binary.name, err, strings.TrimSpace(string(output)))
			return artifact
		}
	}
	_ = buildBroker(repo, binDir, tgt, ext)
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
	artifact.Status = "built"
	if tgt.OS != runtime.GOOS || tgt.Arch != runtime.GOARCH {
		artifact.Message = "cross-built on current host; not notarized or product-live verified"
	}
	return artifact
}

func targetEnv(tgt target) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GOOS="+tgt.OS, "GOARCH="+tgt.Arch, "CGO_ENABLED=1")
	return env
}

func buildBroker(repo, binDir string, tgt target, ext string) error {
	brokerDir := filepath.Join(repo, "cmd", "broker")
	if _, err := os.Stat(filepath.Join(brokerDir, "go.mod")); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, "agent-broker"+ext), ".")
	cmd.Dir = brokerDir
	cmd.Env = targetEnv(tgt)
	_, err := cmd.CombinedOutput()
	return err
}

func writeBundleFiles(repo, stage string) error {
	if err := os.WriteFile(filepath.Join(stage, "QUICKSTART.txt"), []byte("Codencer local production release snapshot\n\nRun ./scripts/install.sh --bin-dir ./bin --dry-run first, then codencer setup local --json.\n"), 0644); err != nil {
		return err
	}
	for _, rel := range []string{"README.md", "LICENSE"} {
		if err := copyFile(filepath.Join(repo, rel), filepath.Join(stage, rel)); err != nil && rel == "LICENSE" {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(stage, "scripts"), 0755); err != nil {
		return err
	}
	_ = copyFile(filepath.Join(repo, "scripts", "install.sh"), filepath.Join(stage, "scripts", "install.sh"))
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
		w, err := zw.Create(filepath.ToSlash(rel))
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
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
