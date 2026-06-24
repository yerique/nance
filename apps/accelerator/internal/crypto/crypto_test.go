package crypto

import (
	"os"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := "12345678901234567890123456789012" // exactly 32 bytes
	os.Setenv("NANCE_MASTER_KEY", key)
	defer os.Unsetenv("NANCE_MASTER_KEY")

	cfg, err := NewConfigFromEnv(os.Getenv)
	if err != nil {
		t.Fatalf("failed to load crypto config: %v", err)
	}

	plaintext := []byte("mongodb://root:secret@real-mongo.example.com:27017/mydb?replicaSet=rs0")
	ct, nonce, ver, err := cfg.Encrypt(plaintext, "demo")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if ver != "v1" || len(ct) == 0 || len(nonce) == 0 {
		t.Fatal("unexpected encrypt output")
	}

	dec, err := cfg.Decrypt(ct, nonce, "demo")
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if string(dec) != string(plaintext) {
		t.Errorf("roundtrip mismatch: got %s want %s", dec, plaintext)
	}

	// Redaction
	redacted := RedactMongoURI(string(plaintext))
	if redacted == string(plaintext) {
		t.Error("redaction did not hide credentials")
	}
}

func TestMissingKey(t *testing.T) {
	os.Unsetenv("NANCE_MASTER_KEY")
	_, err := NewConfigFromEnv(os.Getenv)
	if err != ErrNoMasterKey {
		t.Errorf("expected ErrNoMasterKey, got %v", err)
	}
}
