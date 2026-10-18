package security

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCipherRoundTripAndTamperDetection(t *testing.T) {
	dir := t.TempDir()
	cipher, err := LoadCipher(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, masterKeyFile)); err != nil {
		t.Fatalf("master key missing: %v", err)
	}
	encrypted, err := cipher.Encrypt("notification-token")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "notification-token" {
		t.Fatal("plaintext was returned")
	}
	plain, err := cipher.Decrypt(encrypted)
	if err != nil || plain != "notification-token" {
		t.Fatalf("round trip failed: %q %v", plain, err)
	}
	replacement := byte('A')
	if encrypted[0] == replacement {
		replacement = 'B'
	}
	if _, err := cipher.Decrypt(string(replacement) + encrypted[1:]); err == nil {
		t.Fatal("modified ciphertext was accepted")
	}
}

func TestMaskAndReadOnlyDataGuard(t *testing.T) {
	if got := Mask("8986001234567890123", 6, 4); got != "898600****0123" {
		t.Fatalf("unexpected mask %q", got)
	}
	withoutDevice := Check(context.Background(), nil)
	if withoutDevice.State != "safe" || len(withoutDevice.Findings) != 0 {
		t.Fatalf("no-device guard should be safe: %#v", withoutDevice)
	}
	active := Check(context.Background(), []DeviceNetwork{{DeviceID: "x", WDSStatus: "connected"}})
	if active.State != "critical" || active.WDSStatus != "connected" {
		t.Fatalf("active packet status was not surfaced: %#v", active)
	}
}
