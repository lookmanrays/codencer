package connector

import (
	"encoding/base64"
	"testing"
)

func TestEnsureKeypair_PersistsExistingIdentity(t *testing.T) {
	cfg := &Config{}
	if err := EnsureKeypair(cfg); err != nil {
		t.Fatal(err)
	}
	privateKey := cfg.PrivateKey
	publicKey := cfg.PublicKey
	if privateKey == "" || publicKey == "" {
		t.Fatal("expected keypair to be generated")
	}

	if err := EnsureKeypair(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.PrivateKey != privateKey || cfg.PublicKey != publicKey {
		t.Fatal("expected existing keypair to remain stable")
	}

	if _, err := base64.StdEncoding.DecodeString(cfg.PrivateKey); err != nil {
		t.Fatalf("expected private key to be base64 encoded: %v", err)
	}
}
