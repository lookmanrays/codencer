package cloud

import "testing"

func TestLoadConfigDefaultAndEnvOverrides(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBPath == "" {
		t.Fatal("expected default db path")
	}

	t.Setenv("CODENCER_CLOUD_DB_PATH", "/tmp/codencer-cloud.db")
	t.Setenv("CODENCER_CLOUD_MASTER_KEY", "cloud-master-key")
	cfg, err = LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBPath != "/tmp/codencer-cloud.db" {
		t.Fatalf("expected db path override, got %q", cfg.DBPath)
	}
	if cfg.MasterKey != "cloud-master-key" {
		t.Fatalf("expected master key override, got %q", cfg.MasterKey)
	}
}
