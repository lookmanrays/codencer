package cloud

import "testing"

func TestSecretBoxEncryptDecrypt(t *testing.T) {
	box, err := NewSecretBox("cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, err := box.Encrypt([]byte("installation-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == "" {
		t.Fatal("expected ciphertext")
	}
	if ciphertext == "installation-secret" {
		t.Fatal("expected ciphertext to differ from plaintext")
	}

	plaintext, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "installation-secret" {
		t.Fatalf("unexpected plaintext: %q", plaintext)
	}

	otherBox, err := NewSecretBox("different-master-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherBox.Decrypt(ciphertext); err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}

func TestSecretBoxRequiresMasterKey(t *testing.T) {
	if _, err := NewSecretBox(""); err == nil {
		t.Fatal("expected missing master key to fail")
	}
	var box *SecretBox
	if _, err := box.Encrypt([]byte("secret")); err == nil {
		t.Fatal("expected nil secret box encrypt to fail")
	}
}
