// Package qmi is the real, receive-only QMI adapter.
package qmi

import (
	"context"

	"qmi-web/internal/device"
)

type Backend struct {
	allowedVIDs []string
	controlPath string
}

// NewBackend can be constrained to one explicit control node. The optional
// path is used by the real-validation compose file so a test never picks an
// arbitrary cdc-wdm device when multiple modems are present.
func NewBackend(allowedVIDs []string, controlPath ...string) *Backend {
	b := &Backend{allowedVIDs: append([]string(nil), allowedVIDs...)}
	if len(controlPath) > 0 {
		b.controlPath = controlPath[0]
	}
	return b
}

func (b *Backend) Name() string { return "qmi" }

func (b *Backend) Scan(ctx context.Context) ([]device.Device, error) {
	return device.ScanSystem(ctx, b.allowedVIDs, b.controlPath)
}
