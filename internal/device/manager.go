package device

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mode string
	real Backend
	mock Backend

	mu         sync.RWMutex
	active     Backend
	devices    []Device
	lastScan   error
	lastScanAt time.Time
}

func NewManager(mode string, real, mock Backend) (*Manager, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}
	if real == nil || mock == nil {
		return nil, fmt.Errorf("both real and mock backends are required")
	}
	switch mode {
	case "auto", "qmi", "mock":
	default:
		return nil, fmt.Errorf("unsupported backend %q", mode)
	}
	return &Manager{mode: mode, real: real, mock: mock}, nil
}

func (m *Manager) Scan(ctx context.Context) ([]Device, error) {
	var backend Backend
	switch m.mode {
	case "mock":
		backend = m.mock
	default:
		// Auto never silently becomes mock. A missing device is a real no-device
		// state, not a fabricated modem.
		backend = m.real
	}
	devices, err := backend.Scan(ctx)
	m.mu.Lock()
	m.active, m.devices, m.lastScan, m.lastScanAt = backend, append([]Device(nil), devices...), err, time.Now().UTC()
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return append([]Device(nil), devices...), nil
}

func (m *Manager) Open(ctx context.Context, id string) (Modem, error) {
	m.mu.RLock()
	backend := m.active
	devices := append([]Device(nil), m.devices...)
	m.mu.RUnlock()
	if backend == nil {
		if _, err := m.Scan(ctx); err != nil {
			return nil, err
		}
		m.mu.RLock()
		backend, devices = m.active, append([]Device(nil), m.devices...)
		m.mu.RUnlock()
	}
	for _, d := range devices {
		if d.ID == id {
			if d.Busy {
				return nil, ErrDeviceBusy
			}
			return backend.Open(ctx, id)
		}
	}
	return nil, ErrNoDevice
}

func (m *Manager) BackendName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.active != nil {
		return m.active.Name()
	}
	if m.mode == "mock" {
		return m.mock.Name()
	}
	return m.real.Name()
}

func (m *Manager) Devices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]Device(nil), m.devices...)
}

func (m *Manager) LastScanError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastScan
}

// LastScanAt reports when the manager last completed a read-only discovery
// attempt. It is zero only before the first scan.
func (m *Manager) LastScanAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastScanAt
}
