package device

import (
	"context"
	"testing"
)

type fakeBackend struct {
	name    string
	devices []Device
}

func (b fakeBackend) Name() string { return b.name }
func (b fakeBackend) Scan(context.Context) ([]Device, error) {
	return append([]Device(nil), b.devices...), nil
}
func (b fakeBackend) Open(context.Context, string) (Modem, error) { return nil, ErrNoDevice }

func TestAutoNeverFallsBackToMock(t *testing.T) {
	manager, err := NewManager("auto", fakeBackend{name: "qmi"}, NewMockBackend())
	if err != nil {
		t.Fatal(err)
	}
	devices, err := manager.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 0 || manager.BackendName() != "qmi" {
		t.Fatalf("auto fabricated mock state: %#v / %s", devices, manager.BackendName())
	}
}

func TestMockMustBeExplicit(t *testing.T) {
	manager, err := NewManager("mock", fakeBackend{name: "qmi"}, NewMockBackend())
	if err != nil {
		t.Fatal(err)
	}
	devices, err := manager.Scan(context.Background())
	if err != nil || len(devices) != 1 {
		t.Fatalf("mock scan failed: %v %#v", err, devices)
	}
	modem, err := manager.Open(context.Background(), devices[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer modem.Close()
	status, err := modem.PacketService(context.Background())
	if err != nil || status.State != "disconnected" {
		t.Fatalf("mock packet status is not a safe observation: %#v %v", status, err)
	}
}
