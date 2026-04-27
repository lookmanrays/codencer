package cloud

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const secretCipherVersion = "v1"

// SecretBox encrypts and decrypts installation secrets with an env/config master key.
type SecretBox struct {
	key [32]byte
}

// NewSecretBox builds an AES-GCM secret box from a non-empty master key.
func NewSecretBox(masterKey string) (*SecretBox, error) {
	if strings.TrimSpace(masterKey) == "" {
		return nil, ErrSecretBoxRequired
	}
	return &SecretBox{key: sha256.Sum256([]byte(masterKey))}, nil
}

// Encrypt returns a versioned ciphertext string suitable for database storage.
func (b *SecretBox) Encrypt(plaintext []byte) (string, error) {
	if b == nil {
		return "", ErrSecretBoxRequired
	}
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	joined := append(nonce, sealed...)
	return secretCipherVersion + ":" + base64.RawStdEncoding.EncodeToString(joined), nil
}

// Decrypt reverses Encrypt and fails closed on malformed or tampered ciphertext.
func (b *SecretBox) Decrypt(ciphertext string) ([]byte, error) {
	if b == nil {
		return nil, ErrSecretBoxRequired
	}
	if !strings.HasPrefix(ciphertext, secretCipherVersion+":") {
		return nil, fmt.Errorf("unsupported secret format")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(ciphertext, secretCipherVersion+":"))
	if err != nil {
		return nil, fmt.Errorf("decode secret ciphertext: %w", err)
	}
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("secret ciphertext too short")
	}
	nonce := raw[:gcm.NonceSize()]
	payload := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return plaintext, nil
}
