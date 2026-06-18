package projectconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ScanProposal struct {
	RepoRoot                string   `json:"repo_root"`
	SuggestedProjectID      string   `json:"suggested_project_id"`
	SuggestedProjectName    string   `json:"suggested_project_name"`
	Languages               []string `json:"languages"`
	Frameworks              []string `json:"frameworks,omitempty"`
	PackageManager          string   `json:"package_manager,omitempty"`
	LikelyTestCommands      []string `json:"likely_test_commands,omitempty"`
	LikelyLintCommands      []string `json:"likely_lint_commands,omitempty"`
	LikelyBuildCommands     []string `json:"likely_build_commands,omitempty"`
	SuggestedForbiddenPaths []string `json:"suggested_forbidden_paths"`
	DetectedFiles           []string `json:"detected_files"`
	Confidence              string   `json:"confidence"`
	Warnings                []string `json:"warnings,omitempty"`
	ReadOnly                bool     `json:"read_only"`
}

func Scan(repoRoot string) (ScanProposal, error) {
	abs, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return ScanProposal{}, err
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return ScanProposal{}, err
	}
	if !info.IsDir() {
		return ScanProposal{}, &os.PathError{Op: "scan", Path: abs, Err: errNotDir{}}
	}

	proposal := ScanProposal{
		RepoRoot:                abs,
		SuggestedProjectID:      InferID(abs),
		SuggestedForbiddenPaths: DefaultForbiddenPaths(),
		ReadOnly:                true,
	}
	proposal.SuggestedProjectName = TitleFromID(proposal.SuggestedProjectID)
	if proposal.SuggestedProjectID == "" {
		proposal.Warnings = append(proposal.Warnings, "could not infer a slug-safe project id from repo directory name")
	}

	addFile := func(path string) bool {
		if exists(filepath.Join(abs, filepath.FromSlash(path))) {
			proposal.DetectedFiles = append(proposal.DetectedFiles, path)
			return true
		}
		return false
	}

	if addFile("package.json") {
		proposal.Languages = appendUnique(proposal.Languages, "javascript")
		proposal.PackageManager = detectNodePackageManager(abs)
		proposal.LikelyTestCommands = appendUnique(proposal.LikelyTestCommands, nodeScriptCommand(abs, proposal.PackageManager, "test"))
		proposal.LikelyLintCommands = appendUnique(proposal.LikelyLintCommands, nodeScriptCommand(abs, proposal.PackageManager, "lint"))
		proposal.LikelyBuildCommands = appendUnique(proposal.LikelyBuildCommands, nodeScriptCommand(abs, proposal.PackageManager, "build"))
		if packageJSONHasDependency(abs, "typescript") {
			proposal.Languages = appendUnique(proposal.Languages, "typescript")
		}
		for _, framework := range []string{"next", "react", "vue", "svelte", "vite"} {
			if packageJSONHasDependency(abs, framework) {
				proposal.Frameworks = appendUnique(proposal.Frameworks, framework)
			}
		}
	}
	for _, lock := range []string{"pnpm-lock.yaml", "yarn.lock", "package-lock.json"} {
		addFile(lock)
	}
	if addFile("go.mod") {
		proposal.Languages = appendUnique(proposal.Languages, "go")
		proposal.LikelyTestCommands = appendUnique(proposal.LikelyTestCommands, "go test ./...")
		proposal.LikelyBuildCommands = appendUnique(proposal.LikelyBuildCommands, "go build ./...")
	}
	if addFile("Cargo.toml") {
		proposal.Languages = appendUnique(proposal.Languages, "rust")
		proposal.LikelyTestCommands = appendUnique(proposal.LikelyTestCommands, "cargo test")
		proposal.LikelyBuildCommands = appendUnique(proposal.LikelyBuildCommands, "cargo build")
	}
	if addFile("pyproject.toml") {
		proposal.Languages = appendUnique(proposal.Languages, "python")
		proposal.LikelyTestCommands = appendUnique(proposal.LikelyTestCommands, "pytest")
	}
	if addFile("Makefile") {
		proposal.LikelyTestCommands = appendUnique(proposal.LikelyTestCommands, "make test")
		proposal.LikelyBuildCommands = appendUnique(proposal.LikelyBuildCommands, "make build")
	}
	if addFile("Dockerfile") {
		proposal.Frameworks = appendUnique(proposal.Frameworks, "docker")
	}
	if addFile("docker-compose.yml") {
		proposal.Frameworks = appendUnique(proposal.Frameworks, "docker-compose")
	}
	if addFile("Taskfile.yml") {
		proposal.LikelyTestCommands = appendUnique(proposal.LikelyTestCommands, "task test")
		proposal.LikelyBuildCommands = appendUnique(proposal.LikelyBuildCommands, "task build")
	}
	if addFile("justfile") {
		proposal.LikelyTestCommands = appendUnique(proposal.LikelyTestCommands, "just test")
		proposal.LikelyBuildCommands = appendUnique(proposal.LikelyBuildCommands, "just build")
	}
	if exists(filepath.Join(abs, ".github", "workflows")) {
		proposal.DetectedFiles = append(proposal.DetectedFiles, ".github/workflows")
	}
	if addFile("prisma/schema.prisma") {
		proposal.Frameworks = appendUnique(proposal.Frameworks, "prisma")
	}

	sort.Strings(proposal.DetectedFiles)
	if len(proposal.DetectedFiles) == 0 {
		proposal.Confidence = "low"
		proposal.Warnings = append(proposal.Warnings, "no known project files detected")
	} else if len(proposal.Languages) == 0 {
		proposal.Confidence = "medium"
	} else {
		proposal.Confidence = "high"
	}
	return proposal, nil
}

type errNotDir struct{}

func (errNotDir) Error() string { return "not a directory" }

func detectNodePackageManager(root string) string {
	switch {
	case exists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case exists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case exists(filepath.Join(root, "package-lock.json")):
		return "npm"
	default:
		return "npm"
	}
}

func nodeScriptCommand(root, packageManager, script string) string {
	if !packageJSONHasScript(root, script) {
		return ""
	}
	switch packageManager {
	case "pnpm":
		return "pnpm " + script
	case "yarn":
		return "yarn " + script
	default:
		return "npm run " + script
	}
}

func packageJSONHasScript(root, script string) bool {
	raw := readPackageJSON(root)
	scripts, _ := raw["scripts"].(map[string]any)
	_, ok := scripts[script]
	return ok
}

func packageJSONHasDependency(root, dependency string) bool {
	raw := readPackageJSON(root)
	for _, key := range []string{"dependencies", "devDependencies", "peerDependencies"} {
		deps, _ := raw[key].(map[string]any)
		if _, ok := deps[dependency]; ok {
			return true
		}
	}
	return false
}

func readPackageJSON(root string) map[string]any {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return map[string]any{}
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]any{}
	}
	return raw
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func appendUnique(values []string, next string) []string {
	next = strings.TrimSpace(next)
	if next == "" {
		return values
	}
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}
