package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"qmi-web/internal/config"
	"qmi-web/internal/device"
	"qmi-web/internal/events"
	"qmi-web/internal/logging"
)

type orderedModem struct {
	calls  []string
	packet string
}

func (m *orderedModem) Info(context.Context) (device.Info, error) {
	m.calls = append(m.calls, "dms")
	return device.Info{Manufacturer: "test", Model: "model", Revision: "rev"}, nil
}
func (m *orderedModem) SIM(context.Context) (device.SIM, error) {
	m.calls = append(m.calls, "uim")
	return device.SIM{Present: true, Ready: true, PINStatus: "ready"}, nil
}
func (m *orderedModem) Registration(context.Context) (device.Registration, error) {
	m.calls = append(m.calls, "nas")
	return device.Registration{Registered: true, State: "registered", MCC: "460", MNC: "00"}, nil
}
func (m *orderedModem) Signal(context.Context) (device.Signal, error) {
	m.calls = append(m.calls, "nas_signal")
	return device.Signal{Registered: true, Technology: "LTE"}, nil
}
func (m *orderedModem) PacketService(context.Context) (device.PacketService, error) {
	m.calls = append(m.calls, "wds")
	return device.PacketService{State: m.packet}, nil
}
func (m *orderedModem) ListSMS(context.Context) ([]device.SMSRecord, error) {
	m.calls = append(m.calls, "wms_list")
	return []device.SMSRecord{}, nil
}
func (m *orderedModem) ReadSMS(context.Context, uint8, uint32) (device.SMSRecord, error) {
	return device.SMSRecord{}, nil
}
func (m *orderedModem) SubscribeSMS(context.Context) (<-chan device.SMSNotice, device.WMSSubscription, error) {
	m.calls = append(m.calls, "wms_subscribe")
	return make(chan device.SMSNotice), device.WMSSubscription{EventReport: true, IndicationRegister: true, BindSubscription: "not_available_in_qmi_go_v0.6.4"}, nil
}
func (m *orderedModem) Close() error { return nil }

func testValidationApp(t *testing.T, real bool) *App {
	t.Helper()
	cfg := config.Default()
	cfg.Device.RealValidation = real
	if real {
		cfg.Backend.Type = "qmi"
		cfg.Device.ControlPath = "/dev/cdc-wdm0"
	}
	logger, err := logging.New(filepath.Join(t.TempDir(), "qmi-web.jsonl"), "info", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return &App{Config: cfg, Events: events.New(), Log: logger, validation: newQMIValidation(real, cfg.Device.RealValidationWindow.Value())}
}

func TestReadOnlyValidationOrder(t *testing.T) {
	a := testValidationApp(t, true)
	m := &orderedModem{packet: "disconnected"}
	if _, err := a.initializeCandidate(context.Background(), device.Device{ID: "test"}, m); err != nil {
		t.Fatal(err)
	}
	want := []string{"dms", "uim", "nas", "nas_signal", "wds", "wms_list", "wms_subscribe"}
	if !reflect.DeepEqual(m.calls, want) {
		t.Fatalf("read-only QMI order = %#v, want %#v", m.calls, want)
	}
	state := a.ValidationSnapshot()
	if state.DMS != "pass" || state.UIM != "pass" || state.NAS != "pass" || state.WDS != "pass" || state.WMSList != "pass" || state.WMSSubscribe != "pass" || state.WMSSetEventReport != "pass" || state.WMSIndication != "pass" || state.SMS != "waiting" {
		t.Fatalf("unexpected validation state: %#v", state)
	}
}

func TestConnectedWDSStopsBeforeWMS(t *testing.T) {
	a := testValidationApp(t, true)
	m := &orderedModem{packet: "connected"}
	if _, err := a.initializeCandidate(context.Background(), device.Device{ID: "test"}, m); err == nil {
		t.Fatal("connected WDS state was accepted")
	}
	want := []string{"dms", "uim", "nas", "nas_signal", "wds"}
	if !reflect.DeepEqual(m.calls, want) {
		t.Fatalf("WMS ran after connected WDS: %#v", m.calls)
	}
	state := a.ValidationSnapshot()
	if state.WDS != "blocked" || !state.Terminal || state.WMSList != "pending" || state.WMSSubscribe != "pending" {
		t.Fatalf("unsafe WDS state: %#v", state)
	}
}

func TestRealValidationStatusFileIsSanitized(t *testing.T) {
	a := testValidationApp(t, true)
	a.Config.Device.RealValidationStatusFile = filepath.Join(t.TempDir(), "status.json")
	a.setValidation("sms", "waiting", "REAL_SMS_TEST_READY")

	payload, err := os.ReadFile(a.Config.Device.RealValidationStatusFile)
	if err != nil {
		t.Fatal(err)
	}
	var status QMIValidation
	if err := json.Unmarshal(payload, &status); err != nil {
		t.Fatal(err)
	}
	if status.SMS != "waiting" || status.WindowSeconds != int(time.Hour.Seconds()) || status.RemainingSeconds < 1 {
		t.Fatalf("unexpected persisted state: %#v", status)
	}
	for _, forbidden := range []string{"imei", "imsi", "iccid", "sender", "body", "pdu"} {
		if strings.Contains(strings.ToLower(string(payload)), forbidden) {
			t.Fatalf("status payload contains forbidden field %q: %s", forbidden, payload)
		}
	}
}

func TestCriticalDataGuardTerminatesRealValidation(t *testing.T) {
	a := testValidationApp(t, true)
	a.failRealValidationDataGuard()
	state := a.ValidationSnapshot()
	if !state.Terminal || state.Stage != "data_guard" || state.DataGuard != "fail" || state.Detail != "DATA_GUARD_FAIL" {
		t.Fatalf("critical guard state was not terminal: %#v", state)
	}
}

func TestReconnectBackoffIsBounded(t *testing.T) {
	maximum := 30 * time.Second
	got := []time.Duration{time.Second}
	for len(got) < 6 {
		got = append(got, nextReconnectBackoff(got[len(got)-1], maximum))
	}
	want := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, 30 * time.Second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reconnect backoff = %v, want %v", got, want)
	}
}

func TestObservedStorageScanRecordsSMSPipelineAndDedup(t *testing.T) {
	a := testValidationApp(t, true)
	a.setValidation("sms", "waiting", "REAL_SMS_TEST_READY")
	a.recordSMSPipeline(true, false)
	if state := a.ValidationSnapshot(); state.ReadMessage != "pending" || state.SQLite != "pending" {
		t.Fatalf("baseline storage was treated as a real SMS: %#v", state)
	}
	a.recordSMSPipeline(true, true)
	a.markSMSReceived()
	a.recordSMSPipeline(false, true)
	state := a.ValidationSnapshot()
	if state.SMS != "received" || state.ReadMessage != "pass" || state.Decoder != "pass" || state.SQLite != "pass" || state.Dedup != "pass" {
		t.Fatalf("observed SMS pipeline or dedup was not recorded: %#v", state)
	}
}

func TestObservedDuplicateAfterRestartRecordsDedupWithoutNewReceipt(t *testing.T) {
	a := testValidationApp(t, true)
	a.setValidation("sms", "waiting", "REAL_SMS_TEST_READY")
	a.recordSMSPipeline(false, true)
	state := a.ValidationSnapshot()
	if state.SMS != "waiting" || state.Dedup != "pass" || state.ReadMessage != "pass" || state.Decoder != "pass" || state.SQLite != "pass" {
		t.Fatalf("restart duplicate should record pipeline and dedup without a new receipt: %#v", state)
	}
}

func TestWMSSubscriptionAndOwnershipAreAuditable(t *testing.T) {
	a := testValidationApp(t, true)
	a.setWMSSubscription(device.WMSSubscription{EventReport: true, IndicationRegister: true, BindSubscription: "not_available_in_qmi_go_v0.6.4"})
	a.setDeviceOwnership("QMI Web")
	a.recordStorageReconciliation()
	a.recordWMSIndication()
	state := a.ValidationSnapshot()
	if state.WMSSetEventReport != "pass" || state.WMSIndication != "pass" || state.WMSBind == "" || state.DeviceOwnership != "QMI Web" || state.LastReconcile == nil || state.LastIndication == nil {
		t.Fatalf("subscription/ownership record is incomplete: %#v", state)
	}
}
