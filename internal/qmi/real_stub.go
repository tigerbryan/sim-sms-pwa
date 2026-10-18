//go:build !linux

package qmi

import (
	"context"
	"errors"

	"qmi-web/internal/device"
)

func (b *Backend) Open(context.Context, string) (device.Modem, error) {
	return nil, errors.New("real QMI access requires Linux")
}
