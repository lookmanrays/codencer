package connector

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"runtime"

	"agent-bridge/internal/relayproto"
)

func EnsureKeypair(cfg *Config) error {
	if cfg.PrivateKey != "" && cfg.PublicKey != "" {
		return nil
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate connector keypair: %w", err)
	}
	cfg.PrivateKey = base64.StdEncoding.EncodeToString(privateKey)
	cfg.PublicKey = base64.StdEncoding.EncodeToString(publicKey)
	return nil
}

func PublicKeyFromConfig(cfg *Config) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(cfg.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode connector public key: %w", err)
	}
	return ed25519.PublicKey(decoded), nil
}

func PrivateKeyFromConfig(cfg *Config) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode connector private key: %w", err)
	}
	return ed25519.PrivateKey(decoded), nil
}

func SignChallenge(cfg *Config, challengeID, nonce string) (string, error) {
	privateKey, err := PrivateKeyFromConfig(cfg)
	if err != nil {
		return "", err
	}
	payload := []byte(challengeID + ":" + nonce + ":" + cfg.ConnectorID + ":" + cfg.MachineID)
	signature := ed25519.Sign(privateKey, payload)
	return base64.StdEncoding.EncodeToString(signature), nil
}

func CurrentMachineMetadata() relayproto.MachineMetadata {
	hostname, _ := os.Hostname()
	return relayproto.MachineMetadata{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}
}
