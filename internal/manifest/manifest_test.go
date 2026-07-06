package manifest

import "testing"

func TestParseYAMLManifestWithDefaults(t *testing.T) {
	raw := []byte(`
version: codencer.io/v1alpha1
kind: RunManifest
metadata:
  name: example
project:
  id: codencer
execution:
  profile: fake-success
  timeout: 30m
tasks:
  - id: inspect
    title: Inspect
    goal: Inspect the repo
    validations:
      - name: docs
        command: test -f README.md
        timeout_seconds: 5
`)
	manifest, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.Project.ID != "codencer" || manifest.Execution.Profile != "fake-success" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !StopOnBlocker(manifest.Policy) || !StopOnFailure(manifest.Policy) {
		t.Fatalf("expected stop defaults to be true: %+v", manifest.Policy)
	}
	if manifest.Policy.Retry.MaxAttempts != 1 {
		t.Fatalf("expected retry max attempts default 1, got %d", manifest.Policy.Retry.MaxAttempts)
	}
	if got, err := TimeoutSeconds(manifest.Execution.Timeout); err != nil || got != 1800 {
		t.Fatalf("timeout seconds = %d err=%v", got, err)
	}
}

func TestParseJSONManifest(t *testing.T) {
	raw := []byte(`{"version":"codencer.io/v1alpha1","kind":"RunManifest","tasks":[{"id":"t1","goal":"Do it"}]}`)
	manifest, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if manifest.Tasks[0].ID != "t1" {
		t.Fatalf("unexpected tasks: %+v", manifest.Tasks)
	}
}

func TestValidateRejectsDependsOn(t *testing.T) {
	raw := []byte(`
version: codencer.io/v1alpha1
kind: RunManifest
tasks:
  - id: t1
    goal: First
  - id: t2
    goal: Second
    depends_on: [t1]
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected depends_on to be rejected")
	}
}

func TestProjectIDPrefersCLI(t *testing.T) {
	manifest := &Manifest{Project: Project{ID: "manifest-project"}}
	if got := ProjectID("cli-project", manifest); got != "cli-project" {
		t.Fatalf("project id = %q", got)
	}
	if got := ProjectID("", manifest); got != "manifest-project" {
		t.Fatalf("project id = %q", got)
	}
}
