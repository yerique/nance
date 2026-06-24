package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrNoMasterKey     = errors.New("NANCE_MASTER_KEY is not set")
	ErrInvalidKeySize  = errors.New("master key must be 32 bytes (base64 encoded 44 chars or raw 32 bytes)")
	ErrInvalidCipher   = errors.New("invalid ciphertext or nonce")
)

// Config holds the master key used for envelope encryption (dev/local).
// In production this will be replaced by a KMS-backed DEK.
type Config struct {
	MasterKey []byte
}

// NewConfigFromEnv loads the master key from NANCE_MASTER_KEY env (expected base64 or 32 raw bytes).
func NewConfigFromEnv(getenv func(string) string) (*Config, error) {
	raw := getenv("NANCE_MASTER_KEY")
	if raw == "" {
		return nil, ErrNoMasterKey
	}

	var key []byte
	var err error

	if len(raw) == 32 {
		key = []byte(raw)
	} else {
		key, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			// Also try raw url encoding or just fail
			key, err = base64.RawStdEncoding.DecodeString(raw)
			if err != nil {
				return nil, fmt.Errorf("failed to decode NANCE_MASTER_KEY as base64: %w", err)
			}
		}
	}

	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}
	return &Config{MasterKey: key}, nil
}

// Encrypt encrypts plaintext for a tenant (tenantID currently unused but reserved for future per-tenant DEK).
func (c *Config) Encrypt(plaintext []byte, tenantID string) (ciphertext, nonce []byte, dekVersion string, err error) {
	if len(c.MasterKey) != 32 {
		return nil, nil, "", ErrInvalidKeySize
	}

	block, err := aes.NewCipher(c.MasterKey)
	if err != nil {
		return nil, nil, "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, "", err
	}

	nonce = make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, "", err
	}

	ciphertext = aesgcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, "v1", nil
}

// Decrypt decrypts the stored ciphertext.
func (c *Config) Decrypt(ciphertext, nonce []byte, tenantID string) ([]byte, error) {
	if len(c.MasterKey) != 32 {
		return nil, ErrInvalidKeySize
	}
	if len(nonce) == 0 || len(ciphertext) == 0 {
		return nil, ErrInvalidCipher
	}

	block, err := aes.NewCipher(c.MasterKey)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(nonce) != aesgcm.NonceSize() {
		return nil, ErrInvalidCipher
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}
	return plaintext, nil
}

// RedactMongoURI returns a safe version of a Mongo URI for logging.
func RedactMongoURI(uri string) string {
	if uri == "" {
		return ""
	}
	// Very simple redaction: hide user:pass
	if idx := strings.Index(uri, "://"); idx != -1 {
		rest := uri[idx+3:]
		if at := strings.Index(rest, "@"); at != -1 {
			return uri[:idx+3] + "***:***@" + rest[at+1:]
		}
	}
	return "mongodb://***"
}
