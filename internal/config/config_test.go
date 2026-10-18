package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadParsesHumanDurationAndKeepsSafetyConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
server:
  address: 127.0.0.1
  port: 7580
backend:
  type: mock
security:
  sms_only: true
device:
  scan_interval: 45s
  allowed_vids: ["2c7c"]
sms:
  poll_on_start: true
  sending_enabled: false
  store_raw_pdu: false
notifications:
  pushplus_enabled: false
logging:
  level: info
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Device.ScanInterval.Value() != 45*time.Second {
		t.Fatalf("scan interval = %s", got.Device.ScanInterval.Value())
	}
	if got.Device.ReconnectMaxBackoff.Value() != 30*time.Second || got.SMS.ReconcileInterval.Value() != time.Minute {
		t.Fatalf("safe reconnect/reconciliation defaults were not retained: %#v", got)
	}
	if !got.Security.SMSOnly || got.SMS.SendingEnable || got.SMS.StoreRawPDU {
		t.Fatal("SMS-only constraints were not preserved")
	}
}

func TestValidateRejectsUnsafeConfiguration(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Config)
	}{
		{"data mode", func(c *Config) { c.Security.SMSOnly = false }},
		{"SMS sending", func(c *Config) { c.SMS.SendingEnable = true }},
		{"raw PDU", func(c *Config) { c.SMS.StoreRawPDU = true }},
		{"fast scan", func(c *Config) { c.Device.ScanInterval = Duration(time.Second) }},
		{"too fast reconnect", func(c *Config) { c.Device.ReconnectMaxBackoff = Duration(time.Second) }},
		{"too slow reconcile", func(c *Config) { c.SMS.ReconcileInterval = Duration(11 * time.Minute) }},
		{"invalid control node", func(c *Config) { c.Device.ControlPath = "/dev/ttyUSB2" }},
		{"status file outside data", func(c *Config) { c.Device.RealValidationStatusFile = "/tmp/status.json" }},
		{"real validation without explicit node", func(c *Config) { c.Backend.Type = "qmi"; c.Device.RealValidation = true }},
		{"real validation on mock", func(c *Config) {
			c.Backend.Type = "mock"
			c.Device.ControlPath = "/dev/cdc-wdm0"
			c.Device.RealValidation = true
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.edit(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("unsafe configuration was accepted")
			}
		})
	}
}

func TestWriteUsesReadableDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, Default()); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "scan_interval: 30s") || !strings.Contains(string(content), "reconnect_max_backoff: 30s") || !strings.Contains(string(content), "reconcile_interval: 1m0s") {
		t.Fatalf("duration was not serialized safely: %s", content)
	}
}
