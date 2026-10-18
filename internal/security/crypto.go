// Package security contains safety controls and encryption helpers.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const masterKeyFile = "secrets/master.key"

type Cipher struct {
	aead cipher.AEAD
}

func LoadCipher(dataDir string) (*Cipher, error) {
	if override := strings.TrimSpace(os.Getenv("QMI_WEB_SECRET_KEY")); override != "" {
		key, err := decodeKey(override)
		if err != nil {
			return nil, fmt.Errorf("invalid QMI_WEB_SECRET_KEY: %w", err)
		}
		return newCipher(key)
	}
	if dataDir == "" {
		return nil, errors.New("data directory is required for master key")
	}
	path := filepath.Join(dataDir, masterKeyFile)
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create secret directory: %w", err)
		}
		key = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("generate master key: %w", err)
		}
		f, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if openErr != nil {
			if errors.Is(openErr, os.ErrExist) {
				key, err = os.ReadFile(path)
			}
			if err != nil {
				return nil, fmt.Errorf("create master key: %w", err)
			}
		} else {
			_, writeErr := f.Write(key)
			closeErr := f.Close()
			if writeErr != nil {
				return nil, fmt.Errorf("write master key: %w", writeErr)
			}
			if closeErr != nil {
				return nil, fmt.Errorf("close master key: %w", closeErr)
			}
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("read master key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be exactly 32 bytes")
	}
	return newCipher(key)
}

func newCipher(key []byte) (*Cipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

func decodeKey(input string) ([]byte, error) {
	if raw, err := base64.RawStdEncoding.DecodeString(input); err == nil && len(raw) == 32 {
		return raw, nil
	}
	if raw, err := base64.StdEncoding.DecodeString(input); err == nil && len(raw) == 32 {
		return raw, nil
	}
	// A passphrase is accepted only for deployments that deliberately inject it
	// as an environment secret. It is never persisted or logged.
	sum := sha256.Sum256([]byte(input))
	return sum[:], nil
}

func (c *Cipher) Encrypt(plain string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(out), nil
}

func (c *Cipher) Decrypt(encoded string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	n := c.aead.NonceSize()
	if len(raw) < n {
		return "", errors.New("ciphertext is too short")
	}
	out, err := c.aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func Mask(value string, lead, tail int) string {
	if value == "" {
		return ""
	}
	if lead < 0 {
		lead = 0
	}
	if tail < 0 {
		tail = 0
	}
	if len(value) <= lead+tail {
		return strings.Repeat("*", len(value))
	}
	return value[:lead] + "****" + value[len(value)-tail:]
}
