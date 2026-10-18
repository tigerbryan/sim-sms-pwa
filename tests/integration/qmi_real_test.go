//go:build realqmi

package integration_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"qmi-web/internal/qmi"
)

// TestReadOnlyQMITransport is intentionally opt-in. CI runs without the
// realqmi tag and never receives a QMI device. It issues only the receive-only
// QMI Web adapter methods in the same safety order as real-test startup.
func TestReadOnlyQMITransport(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("real QMI requires Linux")
	}
	control := strings.TrimSpace(os.Getenv("QMI_TEST_DEVICE"))
	if control == "" {
		t.Skip("set QMI_TEST_DEVICE=/dev/cdc-wdmX to opt into real hardware")
	}
	if !strings.HasPrefix(control, "/dev/cdc-wdm") {
		t.Fatalf("refusing non-cdc-wdm device %q", control)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	backend := qmi.NewBackend(nil, control)
	devices, err := backend.Scan(ctx)
	if err != nil {
		t.Fatalf("scan QMI device: %v", err)
	}
	if len(devices) != 1 || devices[0].ControlPath != control {
		t.Fatalf("explicit device selection failed: %#v", devices)
	}
	modem, err := backend.Open(ctx, devices[0].ID)
	if err != nil {
		t.Fatalf("open QMI device: %v", err)
	}
	defer func() { _ = modem.Close() }()

	if _, err := modem.Info(ctx); err != nil {
		t.Fatalf("DMS: %v", err)
	}
	sim, err := modem.SIM(ctx)
	if err != nil {
		t.Fatalf("UIM: %v", err)
	}
	if !sim.Present || !sim.Ready {
		t.Skipf("SIM is not ready (PIN/status withheld): present=%t ready=%t", sim.Present, sim.Ready)
	}
	registration, err := modem.Registration(ctx)
	if err != nil {
		t.Fatalf("NAS: %v", err)
	}
	if !registration.Registered {
		t.Skipf("NAS registration is %s", registration.State)
	}
	packet, err := modem.PacketService(ctx)
	if err != nil {
		t.Fatalf("read-only WDS status: %v", err)
	}
	if strings.ToLower(packet.State) != "disconnected" {
		t.Fatalf("WDS must be disconnected, got %q", packet.State)
	}
	if _, err := modem.ListSMS(ctx); err != nil {
		t.Fatalf("WMS list: %v", err)
	}
	subscribeCtx, stopSubscribe := context.WithCancel(ctx)
	defer stopSubscribe()
	if _, _, err := modem.SubscribeSMS(subscribeCtx); err != nil {
		t.Fatalf("WMS subscribe: %v", err)
	}
	// Do not wait for, create, delete, or send a message here. The bounded
	// external-SMS window belongs to the manually supervised real-test run.
}
