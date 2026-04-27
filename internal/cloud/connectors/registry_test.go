package connectors

import "testing"

func TestRegistryDefaults(t *testing.T) {
	r := NewRegistry()
	if got := r.Names(); len(got) != 5 || got[0] != ProviderGitHub || got[1] != ProviderGitLab || got[2] != ProviderJira || got[3] != ProviderLinear || got[4] != ProviderSlack {
		t.Fatalf("unexpected registry names: %#v", got)
	}
	if _, ok := r.Get(ProviderGitHub); !ok {
		t.Fatalf("github connector missing")
	}
	if _, ok := r.Get(ProviderGitLab); !ok {
		t.Fatalf("gitlab connector missing")
	}
	if _, ok := r.Get(ProviderJira); !ok {
		t.Fatalf("jira connector missing")
	}
	if _, ok := r.Get(ProviderLinear); !ok {
		t.Fatalf("linear connector missing")
	}
	if _, ok := r.Get(ProviderSlack); !ok {
		t.Fatalf("slack connector missing")
	}
}

func TestRegistryRegisterRejectsDuplicates(t *testing.T) {
	r := &Registry{connectors: map[Provider]Connector{}}
	r.MustRegister(NewGitHubConnector(nil))
	if err := r.Register(NewGitHubConnector(nil)); err == nil {
		t.Fatalf("expected duplicate registration to fail")
	}
}
