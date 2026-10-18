package device

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"sync"
	"time"
)

type MockBackend struct {
	mu     sync.Mutex
	device Device
}

func NewMockBackend() *MockBackend {
	return &MockBackend{device: Device{
		ID: "mock-quectel-eg25", ControlPath: "mock://cdc-wdm0", Driver: "mock",
		USBVID: "2c7c", USBPID: "0125", Manufacturer: "Quectel",
		Product: "EG25-G Mock", NetworkInterface: "wwan0", Status: "available",
	}}
}

func (m *MockBackend) Name() string { return "mock" }

func (m *MockBackend) Scan(context.Context) ([]Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []Device{m.device}, nil
}

func (m *MockBackend) Open(_ context.Context, id string) (Modem, error) {
	if id != m.device.ID {
		return nil, ErrNoDevice
	}
	raw, _ := hex.DecodeString("079144872000302320048102020000625061028204401AD9775D0E72D7DBE2B21C949E8360B75A4E7683D16AB71B")
	return &mockModem{device: m.device, sms: make(chan SMSNotice, 4), stop: make(chan struct{}), records: []SMSRecord{{StorageType: 1, StorageIndex: 1, Raw: raw, ReceivedAt: time.Now().UTC()}}}, nil
}

type mockModem struct {
	device  Device
	sms     chan SMSNotice
	stop    chan struct{}
	once    sync.Once
	mu      sync.Mutex
	records []SMSRecord
}

func (m *mockModem) Info(context.Context) (Info, error) {
	return Info{Manufacturer: "Quectel", Model: "EG25-G Mock", Revision: "mock-0.1"}, nil
}

func (m *mockModem) SIM(context.Context) (SIM, error) {
	return SIM{Present: true, Ready: true, PINStatus: "ready", Operator: "Mock Telecom", MCC: "001", MNC: "01", IMSI: "001010123456789", ICCID: "8986001234567890123", Phone: "+15550000001", Registered: true}, nil
}

func (m *mockModem) Signal(context.Context) (Signal, error) {
	seconds := time.Now().Unix() % 8
	return Signal{RSSI: "-67 dBm", RSRP: "-93 dBm", RSRQ: "-10 dB", SINR: strconv.FormatInt(14+seconds/2, 10) + " dB", Technology: "LTE", PLMN: "00101", Registered: true}, nil
}

func (m *mockModem) Registration(context.Context) (Registration, error) {
	return Registration{Registered: true, State: "registered", Technology: "LTE", MCC: "001", MNC: "01"}, nil
}

func (m *mockModem) PacketService(context.Context) (PacketService, error) {
	return PacketService{State: "disconnected"}, nil
}

func (m *mockModem) ListSMS(context.Context) ([]SMSRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]SMSRecord(nil), m.records...), nil
}

func (m *mockModem) ReadSMS(_ context.Context, storage uint8, index uint32) (SMSRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, record := range m.records {
		if record.StorageType == storage && record.StorageIndex == index {
			return record, nil
		}
	}
	return SMSRecord{}, errors.New("mock storage does not contain this message")
}

func (m *mockModem) SubscribeSMS(ctx context.Context) (<-chan SMSNotice, WMSSubscription, error) {
	status := WMSSubscription{EventReport: true, IndicationRegister: true, BindSubscription: "not-required-for-mock"}
	interval, _ := time.ParseDuration(os.Getenv("QMI_WEB_MOCK_SMS_INTERVAL"))
	if interval <= 0 {
		return m.sms, status, nil
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stop:
				return
			case <-ticker.C:
				m.mu.Lock()
				next := uint32(len(m.records) + 1)
				template := m.records[0]
				template.StorageIndex, template.ReceivedAt = next, time.Now().UTC()
				m.records = append(m.records, template)
				m.mu.Unlock()
				select {
				case m.sms <- SMSNotice{}:
				default:
				}
			}
		}
	}()
	return m.sms, status, nil
}

func (m *mockModem) Close() error {
	m.once.Do(func() { close(m.stop) })
	return nil
}
