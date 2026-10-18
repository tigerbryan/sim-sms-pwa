// Package config loads the non-secret application configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "/config/config.yaml"

// Duration preserves human-readable duration values such as "30s" in YAML.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d Duration) Value() time.Duration {
	return time.Duration(d)
}

type Config struct {
	Server struct {
		Address string `yaml:"address"`
		Port    int    `yaml:"port"`
	} `yaml:"server"`
	Backend struct {
		Type string `yaml:"type"`
	} `yaml:"backend"`
	Security struct {
		SMSOnly bool `yaml:"sms_only"`
	} `yaml:"security"`
	Device struct {
		ScanInterval             Duration `yaml:"scan_interval"`
		ReconnectMaxBackoff      Duration `yaml:"reconnect_max_backoff"`
		AllowedVIDs              []string `yaml:"allowed_vids"`
		ControlPath              string   `yaml:"control_path"`
		RealValidation           bool     `yaml:"real_validation"`
		RealValidationWindow     Duration `yaml:"real_validation_window"`
		RealValidationStatusFile string   `yaml:"real_validation_status_file"`
	} `yaml:"device"`
	SMS struct {
		PollOnStart       bool     `yaml:"poll_on_start"`
		ReconcileInterval Duration `yaml:"reconcile_interval"`
		SendingEnable     bool     `yaml:"sending_enabled"`
		StoreRawPDU       bool     `yaml:"store_raw_pdu"`
	} `yaml:"sms"`
	Notifications struct {
		PushPlusEnabled bool `yaml:"pushplus_enabled"`
	} `yaml:"notifications"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
}

func Default() Config {
	var c Config
	c.Server.Address = "0.0.0.0"
	c.Server.Port = 7580
	c.Backend.Type = "auto"
	c.Security.SMSOnly = true
	c.Device.ScanInterval = Duration(30 * time.Second)
	c.Device.ReconnectMaxBackoff = Duration(30 * time.Second)
	c.Device.AllowedVIDs = []string{}
	c.Device.ControlPath = ""
	c.Device.RealValidation = false
	c.Device.RealValidationWindow = Duration(time.Hour)
	c.Device.RealValidationStatusFile = ""
	c.SMS.PollOnStart = true
	c.SMS.ReconcileInterval = Duration(time.Minute)
	c.SMS.SendingEnable = false
	c.SMS.StoreRawPDU = false
	c.Notifications.PushPlusEnabled = false
	c.Logging.Level = "info"
	return c
}

func Load(path string) (Config, error) {
	c := Default()
	if path == "" {
		path = DefaultPath
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return c, fmt.Errorf("create config directory: %w", err)
		}
		if err := Write(path, c); err != nil {
			return c, err
		}
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("parse config: %w", err)
	}
	return c, c.Validate()
}

func Write(path string, c Config) error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o640); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port %d", c.Server.Port)
	}
	switch strings.ToLower(c.Backend.Type) {
	case "auto", "qmi", "mock":
	default:
		return fmt.Errorf("unsupported backend %q", c.Backend.Type)
	}
	if !c.Security.SMSOnly {
		return errors.New("SMS-only safety mode cannot be disabled")
	}
	if c.SMS.SendingEnable {
		return errors.New("SMS sending is not available in SMS-only mode")
	}
	if c.SMS.StoreRawPDU {
		return errors.New("raw PDU storage is not enabled in SMS-only mode")
	}
	if c.Device.ControlPath != "" {
		clean := filepath.Clean(c.Device.ControlPath)
		if clean != c.Device.ControlPath || filepath.Dir(clean) != "/dev" || !strings.HasPrefix(filepath.Base(clean), "cdc-wdm") {
			return errors.New("device control_path must be one /dev/cdc-wdmX node")
		}
	}
	if c.Device.RealValidation {
		if strings.ToLower(c.Backend.Type) != "qmi" {
			return errors.New("real validation requires the qmi backend")
		}
		if c.Device.ControlPath == "" {
			return errors.New("real validation requires an explicit /dev/cdc-wdmX control_path")
		}
		if c.Device.RealValidationWindow.Value() < time.Minute || c.Device.RealValidationWindow.Value() > time.Hour {
			return errors.New("real validation window must be between 1m and 60m")
		}
	}
	if c.Device.RealValidationStatusFile != "" {
		clean := filepath.Clean(c.Device.RealValidationStatusFile)
		if clean != c.Device.RealValidationStatusFile || !filepath.IsAbs(clean) || (clean != "/data" && !strings.HasPrefix(clean, "/data/")) {
			return errors.New("real validation status file must stay under /data")
		}
	}
	if c.Device.ScanInterval.Value() < 10*time.Second {
		return errors.New("device scan interval must be at least 10s")
	}
	if c.Device.ReconnectMaxBackoff.Value() < 10*time.Second || c.Device.ReconnectMaxBackoff.Value() > 5*time.Minute {
		return errors.New("device reconnect_max_backoff must be between 10s and 5m")
	}
	if c.SMS.ReconcileInterval.Value() < 30*time.Second || c.SMS.ReconcileInterval.Value() > 10*time.Minute {
		return errors.New("SMS reconcile_interval must be between 30s and 10m")
	}
	return nil
}

func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}
