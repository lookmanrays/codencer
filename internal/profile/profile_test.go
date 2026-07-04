package profile

import "testing"

func TestResolveDefaultsCodexToWorkspaceProfile(t *testing.T) {
	resolved, err := Resolve(ResolveOptions{Adapter: "codex"})
	if err != nil {
		t.Fatalf("resolve codex: %v", err)
	}
	if resolved.ProfileID != "codex-workspace" || resolved.DaemonAdapter != "codex" {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}

func TestResolveLegacyProjectProfile(t *testing.T) {
	resolved, err := Resolve(ResolveOptions{ProjectDefaultAdapter: "codex", ProjectProfile: "codex"})
	if err != nil {
		t.Fatalf("resolve legacy profile: %v", err)
	}
	if resolved.ProfileID != "codex-workspace" || resolved.Source != "legacy_adapter_profile" {
		t.Fatalf("unexpected legacy resolution: %+v", resolved)
	}
}

func TestResolveFakeProfileMapsToDaemonAdapter(t *testing.T) {
	resolved, err := Resolve(ResolveOptions{ProfileID: "fake-blocker"})
	if err != nil {
		t.Fatalf("resolve fake profile: %v", err)
	}
	if resolved.Adapter != "fake" || resolved.DaemonAdapter != "fake-blocker" {
		t.Fatalf("unexpected fake resolution: %+v", resolved)
	}
}

func TestResolveExplicitProfileDeterminesAdapter(t *testing.T) {
	resolved, err := Resolve(ResolveOptions{ProjectDefaultAdapter: "codex", ProjectProfile: "codex-workspace", ProfileID: "claude-default"})
	if err != nil {
		t.Fatalf("resolve explicit claude profile with codex project default: %v", err)
	}
	if resolved.Adapter != "claude" || resolved.ProfileID != "claude-default" || resolved.DaemonAdapter != "claude" {
		t.Fatalf("unexpected explicit profile resolution: %+v", resolved)
	}
}

func TestResolveExplicitFullProfile(t *testing.T) {
	resolved, err := Resolve(ResolveOptions{Adapter: "codex", ProfileID: "codex-full"})
	if err != nil {
		t.Fatalf("resolve codex-full: %v", err)
	}
	if resolved.ProfileID != "codex-full" || resolved.Profile.Sandbox != "danger-full-access" {
		t.Fatalf("unexpected full profile resolution: %+v", resolved)
	}
}

func TestDangerousBypassRequiresExplicitAllow(t *testing.T) {
	if _, err := Resolve(ResolveOptions{ProfileID: "codex-danger-bypass"}); err == nil {
		t.Fatal("expected dangerous bypass to require explicit allow")
	}
	if _, err := Resolve(ResolveOptions{ProfileID: "codex-danger-bypass", AllowDangerousBypass: true}); err != nil {
		t.Fatalf("expected explicit dangerous bypass allow to pass: %v", err)
	}
}

func TestResolveRejectsAdapterMismatch(t *testing.T) {
	if _, err := Resolve(ResolveOptions{Adapter: "claude", ProfileID: "codex-workspace"}); err == nil {
		t.Fatal("expected adapter/profile mismatch to fail")
	}
}
