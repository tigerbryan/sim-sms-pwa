// Package app composes the receive-only QMI services.
package app

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"qmi-web/internal/config"
	"qmi-web/internal/database"
	"qmi-web/internal/device"
	"qmi-web/internal/events"
	"qmi-web/internal/logging"
	"qmi-web/internal/notify"
	"qmi-web/internal/security"
	"qmi-web/internal/sms"
	"qmi-web/internal/version"
)

type App struct {
	Config        config.Config
	Store         *database.Store
	Devices       *device.Manager
	Events        *events.Bus
	Log           *logging.Recorder
	Cipher        *security.Cipher
	Notifications *notify.Service
	Pipeline      *sms.Pipeline
	StartedAt     time.Time

	connectMu     sync.Mutex
	mu            sync.RWMutex
	modem         device.Modem
	current       *device.Device
	info          device.Info
	sim           device.SIM
	signal        device.Signal
	reg           device.Registration
	guard         security.GuardState
	validation    QMIValidation
	everConnected bool
}

type Dashboard struct {
	Version        string                 `json:"version"`
	Backend        string                 `json:"backend"`
	DeviceStatus   string                 `json:"device_status"`
	Device         *device.Device         `json:"device,omitempty"`
	QMIStatus      string                 `json:"qmi_status"`
	SIM            SIMSnapshot            `json:"sim"`
	Signal         SignalSnapshot         `json:"signal"`
	SMS            DashboardSMS           `json:"sms"`
	Notifications  DashboardNotifications `json:"notifications"`
	UnreadSMS      int                    `json:"unread_sms"`
	LastSMS        *time.Time             `json:"last_sms,omitempty"`
	PushPlus       notify.PublicConfig    `json:"pushplus"`
	UptimeSeconds  int64                  `json:"uptime_seconds"`
	DatabaseStatus string                 `json:"database_status"`
	SMSOnly        bool                   `json:"sms_only"`
	DataGuard      security.GuardState    `json:"data_guard"`
	QMIValidation  QMIValidation          `json:"qmi_validation"`
}

// DashboardSMS and DashboardNotifications keep the overview response useful
// without requiring the browser to infer state from unrelated endpoints.
type DashboardSMS struct {
	Total  int        `json:"total"`
	Unread int        `json:"unread"`
	Last   *time.Time `json:"last,omitempty"`
}

type DashboardNotifications struct {
	PushPlus notify.PublicConfig `json:"pushplus"`
}

// DevicesSnapshot, SIMSnapshot and SignalSnapshot are the explicit no-device
// API models. They never trigger discovery or attempt to open a modem.
type DevicesSnapshot struct {
	Devices  []device.Device `json:"devices"`
	Backend  string          `json:"backend"`
	ScanTime time.Time       `json:"scan_time"`
}

type SIMSnapshot struct {
	Available  bool   `json:"available"`
	Present    bool   `json:"present"`
	Ready      bool   `json:"ready"`
	PINStatus  string `json:"pin_status"`
	Operator   string `json:"operator"`
	MCC        string `json:"mcc"`
	MNC        string `json:"mnc"`
	IMSI       string `json:"imsi,omitempty"`
	ICCID      string `json:"iccid,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Registered bool   `json:"registered"`
	Roaming    bool   `json:"roaming"`
}

type SignalSnapshot struct {
	Available  bool   `json:"available"`
	RSSI       string `json:"rssi"`
	RSRP       string `json:"rsrp"`
	RSRQ       string `json:"rsrq"`
	SINR       string `json:"sinr"`
	Technology string `json:"technology"`
	PLMN       string `json:"plmn"`
	Roaming    bool   `json:"roaming"`
	Registered bool   `json:"registered"`
}

func New(cfg config.Config, store *database.Store, manager *device.Manager, cipher *security.Cipher, logger *logging.Recorder) *App {
	bus := events.New()
	if logger == nil {
		// The caller normally supplies a logger; API tests can use nil safely.
		logger, _ = logging.New("logs/qmi-web.jsonl", cfg.Logging.Level, bus)
	}
	if logger != nil {
		logger.SetBus(bus)
	}
	a := &App{Config: cfg, Store: store, Devices: manager, Events: bus, Cipher: cipher, Log: logger, StartedAt: time.Now().UTC(), validation: newQMIValidation(cfg.Device.RealValidation, cfg.Device.RealValidationWindow.Value())}
	a.Notifications = notify.NewService(store, cipher, a.DataGuard)
	a.Pipeline = &sms.Pipeline{Store: store, Events: bus, OnComplete: a.onSMS, Source: smsSource(cfg)}
	return a
}

func smsSource(cfg config.Config) string {
	if cfg.Device.RealValidation || strings.EqualFold(cfg.Backend.Type, "qmi") {
		return "modem"
	}
	return "test_fixture"
}

func (a *App) Start(ctx context.Context) {
	go a.Notifications.Run(ctx)
	go a.connectionLoop(ctx)
	go a.reconcileLoop(ctx)
	go a.guardLoop(ctx)
	if a.Config.Device.RealValidation {
		a.persistValidation(a.ValidationSnapshot())
		a.startRealValidationWindow(ctx.Done())
	}
}

func (a *App) Shutdown() error {
	a.mu.Lock()
	modem := a.modem
	a.modem, a.current = nil, nil
	a.mu.Unlock()
	if modem != nil {
		return modem.Close()
	}
	return nil
}

func (a *App) connectionLoop(ctx context.Context) {
	delay := time.Duration(0)
	backoff := time.Second
	for {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		connected := a.scanAndConnect(ctx)
		if connected {
			delay, backoff = a.scanInterval(), time.Second
			continue
		}
		delay = backoff
		backoff = nextReconnectBackoff(backoff, a.Config.Device.ReconnectMaxBackoff.Value())
	}
}

func nextReconnectBackoff(current, maximum time.Duration) time.Duration {
	if current <= 0 {
		return time.Second
	}
	for _, candidate := range []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second} {
		if candidate > current {
			if candidate > maximum {
				return maximum
			}
			return candidate
		}
	}
	return maximum
}

func (a *App) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(a.Config.SMS.ReconcileInterval.Value())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.syncSMS(ctx, true)
		}
	}
}

func (a *App) guardLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			guard := a.DataGuard(ctx)
			if a.Config.Device.RealValidation {
				if guard.State == "critical" {
					a.recordDataGuard("fail", guard.WDSStatus)
					a.failRealValidationDataGuard()
					return
				}
				a.recordDataGuard("pass", guard.WDSStatus)
				a.refreshValidationWindow()
			}
			if guard.State == "critical" {
				data := map[string]any{"findings": guard.Findings, "wds_status": guard.WDSStatus}
				a.Events.Publish(events.Event{Type: events.DataConnectionDetected, Data: data})
				a.Events.Publish(events.Event{Type: events.SecurityWarning, Data: data})
				a.Log.Warn("SMS-only data guard is critical", map[string]any{"findings": len(guard.Findings)})
				if !guard.HasFinding("CellularDefaultRoute") {
					_, err := a.Notifications.Enqueue(ctx, notify.NotificationEvent{
						Kind: "security", Title: "QMI Web security warning",
						Body:     "Data Guard detected a cellular data connection. Review the local dashboard.",
						DedupKey: "security:data-connection",
					})
					if err != nil {
						a.Log.Warn("security notification enqueue failed", map[string]any{"error": err.Error()})
					}
				}
			}
		}
	}
}

func (a *App) scanAndConnect(ctx context.Context) bool {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	if a.Config.Device.RealValidation && a.ValidationSnapshot().Terminal {
		return true
	}
	devices, err := a.Devices.Scan(ctx)
	if err != nil {
		a.setValidation("device_open", "fail", "DEVICE_SCAN_FAILED")
		a.Log.Warn("device scan failed", map[string]any{"error": err.Error()})
		return false
	}
	if len(devices) == 0 {
		a.setValidation("device_open", "pending", "NO_QMI_DEVICE")
		a.disconnect()
		return false
	}
	if a.Config.Device.RealValidation && len(devices) != 1 {
		a.setValidation("device_open", "blocked", "MULTIPLE_CDC_WDM_TARGETS")
		a.Log.Warn("real validation requires exactly one explicitly selected QMI device", map[string]any{"count": len(devices)})
		return false
	}
	a.mu.RLock()
	current, modem := a.current, a.modem
	a.mu.RUnlock()
	if current != nil && modem != nil && containsDevice(devices, current.ID) {
		healthCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, healthErr := modem.PacketService(healthCtx)
		cancel()
		if healthErr == nil {
			return true
		}
		a.Log.Warn("QMI health probe failed; reconnecting", map[string]any{"device": current.ControlPath, "error": healthErr.Error()})
		a.disconnect()
	} else if current != nil {
		// A partially initialized connection must never be treated as healthy.
		// Close it before the next read-only open attempt instead of dereferencing
		// a nil modem or retaining an ambiguous device owner.
		a.Log.Warn("QMI connection state is incomplete; reconnecting", map[string]any{"device": current.ControlPath})
		a.disconnect()
	}
	a.disconnect()
	first := devices[0]
	a.setValidation("device_open", "running", "")
	opened, err := a.Devices.Open(ctx, first.ID)
	if err != nil {
		a.setValidation("device_open", "fail", "QMI_OPEN_FAILED")
		a.Log.Warn("QMI device remains unavailable", map[string]any{"device": first.ControlPath, "error": err.Error()})
		return false
	}
	a.setValidation("device_open", "pass", "")
	notices, err := a.initializeCandidate(ctx, first, opened)
	if err != nil {
		_ = opened.Close()
		return false
	}
	first.Status = "connected"
	a.mu.Lock()
	wasConnected := a.everConnected
	a.everConnected = true
	a.modem, a.current = opened, &first
	a.mu.Unlock()
	if wasConnected {
		a.recordReconnect()
	}
	a.setDeviceOwnership("QMI Web")
	a.Events.Publish(events.Event{Type: events.DeviceConnected, Data: map[string]any{"device_id": first.ID}})
	a.Log.Info("QMI device connected after read-only validation", map[string]any{"device": first.ControlPath, "real_validation": a.Config.Device.RealValidation})
	go a.subscribe(ctx, notices)
	return true
}

func (a *App) disconnect() {
	a.mu.Lock()
	modem := a.modem
	wasConnected := a.current != nil
	a.modem, a.current = nil, nil
	a.mu.Unlock()
	if modem != nil {
		_ = modem.Close()
	}
	if wasConnected {
		a.Events.Publish(events.Event{Type: events.DeviceDisconnected})
		a.Log.Info("QMI device disconnected", nil)
		a.setDeviceOwnership("released")
	}
}

func (a *App) initializeCandidate(ctx context.Context, candidate device.Device, modem device.Modem) (<-chan device.SMSNotice, error) {
	a.setValidation("dms", "running", "")
	info, err := modem.Info(ctx)
	if err != nil {
		a.setValidation("dms", "fail", "DMS_FAILED")
		a.Log.Warn("DMS gate failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	a.mu.Lock()
	a.info = info
	a.mu.Unlock()
	a.setValidation("dms", "pass", "")

	a.setValidation("uim", "running", "")
	sim, err := modem.SIM(ctx)
	if err != nil {
		a.setValidation("uim", "fail", "UIM_FAILED")
		a.Log.Warn("UIM gate failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	if !sim.Present {
		a.setValidation("uim", "blocked", "SIM_ABSENT")
		return nil, fmt.Errorf("SIM absent")
	}
	if !sim.Ready {
		detail := "SIM_NOT_READY"
		if strings.Contains(strings.ToLower(sim.PINStatus), "pin") {
			detail = "SIM_PIN_REQUIRED"
		}
		a.setValidation("uim", "blocked", detail)
		return nil, fmt.Errorf("SIM is not ready")
	}
	a.setValidation("uim", "pass", "")

	reg, err := a.waitForRegistration(ctx, modem)
	if err != nil {
		a.setValidation("nas", "blocked", "NETWORK_REGISTRATION_TIMEOUT")
		a.Log.Warn("NAS registration gate failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	sim.MCC, sim.MNC, sim.Registered, sim.Roaming = reg.MCC, reg.MNC, reg.Registered, reg.Roaming
	sim.Operator = reg.MCC + reg.MNC
	a.mu.Lock()
	a.sim, a.reg = sim, reg
	a.mu.Unlock()
	if signal, signalErr := modem.Signal(ctx); signalErr == nil {
		a.mu.Lock()
		a.signal = signal
		a.mu.Unlock()
		a.Events.Publish(events.Event{Type: events.SignalChanged})
	} else {
		// Signal metrics are optional across RATs. Registration remains a valid
		// NAS gate when a particular metric is unavailable.
		a.Log.Warn("NAS signal metrics unavailable", map[string]any{"error": signalErr.Error()})
	}
	a.Events.Publish(events.Event{Type: events.SIMReady})
	a.Events.Publish(events.Event{Type: events.RegistrationChanged})
	a.setValidation("nas", "pass", "")

	a.setValidation("wds", "running", "")
	packet, err := modem.PacketService(ctx)
	if err != nil {
		a.setValidation("wds", "fail", "WDS_STATUS_FAILED")
		a.Log.Warn("read-only WDS gate failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	wdsStatus := strings.ToLower(strings.TrimSpace(packet.State))
	a.setWDSStatus(wdsStatus)
	if wdsStatus != "disconnected" {
		a.setValidation("wds", "blocked", "WDS_NOT_DISCONNECTED")
		a.Log.Warn("WDS reports a non-disconnected state; WMS is not started", map[string]any{"status": wdsStatus})
		return nil, fmt.Errorf("WDS status is %s", wdsStatus)
	}
	a.setValidation("wds", "pass", "")

	a.setValidation("wms_list", "running", "")
	records, err := modem.ListSMS(ctx)
	if err != nil {
		a.setValidation("wms_list", "fail", "WMS_LIST_FAILED")
		a.Log.Warn("WMS storage scan gate failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	a.recordStorageReconciliation()
	a.ingestRecords(ctx, candidate.ID, records, false)
	a.setValidation("wms_list", "pass", "")

	a.setValidation("wms_subscribe", "running", "")
	notices, subscription, err := modem.SubscribeSMS(ctx)
	if err != nil {
		a.setValidation("wms_subscribe", "fail", "WMS_SUBSCRIBE_FAILED")
		a.Log.Warn("WMS event subscription gate failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	a.setWMSSubscription(subscription)
	a.setValidation("wms_subscribe", "pass", "")
	if a.Config.Device.RealValidation {
		a.setValidation("sms", "waiting", "REAL_SMS_TEST_READY")
	}
	return notices, nil
}

func (a *App) waitForRegistration(ctx context.Context, modem device.Modem) (device.Registration, error) {
	deadline := time.Now()
	if a.Config.Device.RealValidation {
		deadline = deadline.Add(120 * time.Second)
	}
	var lastErr error
	for {
		reg, err := modem.Registration(ctx)
		if err == nil && reg.Registered {
			return reg, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("registration state is %s", reg.State)
		}
		if !a.Config.Device.RealValidation || !time.Now().Before(deadline) {
			return device.Registration{}, lastErr
		}
		a.setValidation("nas", "running", "WAITING_FOR_REGISTRATION")
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return device.Registration{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func (a *App) subscribe(ctx context.Context, ch <-chan device.SMSNotice) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			a.recordWMSIndication()
			// The callback is intentionally decoupled from the storage scan.
			go a.syncSMS(ctx, true)
		}
	}
}

func (a *App) syncSMS(ctx context.Context, observed bool) {
	a.mu.RLock()
	modem, current := a.modem, a.current
	a.mu.RUnlock()
	if modem == nil || current == nil {
		return
	}
	a.recordStorageReconciliation()
	records, err := modem.ListSMS(ctx)
	if err != nil {
		a.Log.Warn("SMS storage scan failed", map[string]any{"device": current.ID, "error": err.Error()})
		return
	}
	a.ingestRecords(ctx, current.ID, records, observed)
}

func (a *App) ingestRecords(ctx context.Context, deviceID string, records []device.SMSRecord, observed bool) {
	imported := 0
	for _, record := range records {
		_, saved, err := a.Pipeline.Ingest(ctx, deviceID, record)
		if err != nil {
			a.Log.Warn("SMS PDU was not persisted", map[string]any{"device": deviceID, "error": err.Error()})
			continue
		}
		a.recordSMSPipeline(saved, observed)
		if saved {
			imported++
			a.Log.Info("SMS persisted", map[string]any{"device": deviceID, "storage_index": record.StorageIndex})
			if observed {
				a.markSMSReceived()
			}
		}
	}
	a.recordSMSScan(len(records), imported)
}

func (a *App) onSMS(ctx context.Context, message database.SMSMessage) {
	if a.Config.Device.RealValidation {
		guard := a.DataGuard(ctx)
		if guard.State == "critical" {
			a.Log.Warn("real SMS notification blocked by data guard", map[string]any{"sms_id": message.ID})
			return
		}
	}
	body := "设备: " + message.DeviceID + "\n发送人: " + maskPhone(message.Sender) + "\n时间: " + message.Timestamp.UTC().Format(time.RFC3339) + "\n短信正文: " + truncate(message.Body, 1200)
	_, err := a.Notifications.Enqueue(ctx, notify.NotificationEvent{
		Kind: "sms", Title: "新短信", Body: body, DedupKey: fmt.Sprintf("sms:%d", message.ID),
		Sender: message.Sender, Text: message.Body,
	})
	if err != nil {
		a.Log.Warn("SMS notification enqueue failed", map[string]any{"sms_id": message.ID, "error": err.Error()})
	}
}

func (a *App) Dashboard(ctx context.Context) (Dashboard, error) {
	unread, err := a.Store.UnreadSMSCount(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	_, total, err := a.Store.ListSMS(ctx, 1, 1, "", "")
	if err != nil {
		return Dashboard{}, err
	}
	lastSMS, err := a.Store.LatestSMSAt(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	push, err := a.Notifications.Config(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	a.mu.RLock()
	current, sim, signal, validation := a.current, a.sim, a.signal, a.validation
	a.mu.RUnlock()
	status, qmi := "no-device", "unavailable"
	if current != nil {
		status, qmi = "connected", "connected"
	} else if validation.Stage != "" && validation.Stage != "idle" {
		qmi = validation.Stage
	}
	dbStatus := "ready"
	if err := a.Store.Healthy(ctx); err != nil {
		dbStatus = "error"
	}
	return Dashboard{
		Version: version.Version, Backend: a.Devices.BackendName(), DeviceStatus: status, Device: current,
		QMIStatus: qmi, SIM: publicSIM(sim, current != nil), Signal: signalSnapshot(signal, current != nil),
		SMS:           DashboardSMS{Total: total, Unread: unread, Last: lastSMS},
		Notifications: DashboardNotifications{PushPlus: push},
		UnreadSMS:     unread, LastSMS: lastSMS, PushPlus: push,
		UptimeSeconds: int64(time.Since(a.StartedAt).Seconds()), DatabaseStatus: dbStatus,
		SMSOnly: true, DataGuard: a.DataGuard(ctx), QMIValidation: validation,
	}, nil
}

func (a *App) DevicesSnapshot() DevicesSnapshot {
	devices := a.Devices.Devices()
	if devices == nil {
		devices = []device.Device{}
	}
	return DevicesSnapshot{
		Devices: devices, Backend: a.Devices.BackendName(), ScanTime: a.Devices.LastScanAt(),
	}
}

type RuntimeSettings struct {
	Backend      string
	ScanInterval time.Duration
	LogLevel     string
}

func (a *App) RuntimeSettings() RuntimeSettings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return RuntimeSettings{
		Backend: a.Config.Backend.Type, ScanInterval: a.Config.Device.ScanInterval.Value(),
		LogLevel: a.Config.Logging.Level,
	}
}

func (a *App) UpdateRuntimeSettings(scanInterval string, logLevel string) error {
	a.mu.Lock()
	next := a.Config
	if scanInterval != "" {
		parsed, err := time.ParseDuration(scanInterval)
		if err != nil {
			a.mu.Unlock()
			return fmt.Errorf("invalid scan interval: %w", err)
		}
		next.Device.ScanInterval = config.Duration(parsed)
	}
	if logLevel != "" {
		next.Logging.Level = strings.ToLower(strings.TrimSpace(logLevel))
	}
	if err := next.Validate(); err != nil {
		a.mu.Unlock()
		return err
	}
	if a.Log != nil && logLevel != "" {
		if err := a.Log.SetLevel(next.Logging.Level); err != nil {
			a.mu.Unlock()
			return err
		}
	}
	// Only mutate live settings: worker goroutines read immutable configuration fields.
	a.Config.Device.ScanInterval = next.Device.ScanInterval
	a.Config.Logging.Level = next.Logging.Level
	a.mu.Unlock()
	return nil
}

func (a *App) scanInterval() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Config.Device.ScanInterval.Value()
}

func (a *App) SIMSnapshot() SIMSnapshot {
	a.mu.RLock()
	sim, available := a.sim, a.current != nil
	a.mu.RUnlock()
	return publicSIM(sim, available)
}

func (a *App) SignalSnapshot() SignalSnapshot {
	a.mu.RLock()
	signal, available := a.signal, a.current != nil
	a.mu.RUnlock()
	return signalSnapshot(signal, available)
}

func (a *App) DataGuard(ctx context.Context) security.GuardState {
	a.mu.RLock()
	current, modem := a.current, a.modem
	a.mu.RUnlock()
	var input []security.DeviceNetwork
	if current != nil {
		status := "unknown"
		if modem != nil {
			statusCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			if packet, err := modem.PacketService(statusCtx); err == nil && packet.State != "" {
				status = packet.State
			}
			cancel()
		}
		input = append(input, security.DeviceNetwork{DeviceID: current.ID, NetworkInterface: current.NetworkInterface, WDSStatus: status})
	}
	guard := security.Check(ctx, input)
	a.mu.Lock()
	a.guard = guard
	a.mu.Unlock()
	return guard
}

func (a *App) Diagnostics(ctx context.Context) map[string]any {
	a.mu.RLock()
	current, validation := a.current, a.validation
	a.mu.RUnlock()
	ready := a.Store.Healthy(ctx) == nil
	return map[string]any{
		"version": version.Current(), "os": runtime.GOOS, "architecture": runtime.GOARCH,
		"uptime_seconds": int64(time.Since(a.StartedAt).Seconds()), "database_ready": ready,
		"backend": a.Devices.BackendName(), "detected_devices": a.DevicesSnapshot().Devices,
		"active_device": current, "sms_only": true, "guard": a.DataGuard(ctx), "qmi_validation": validation,
	}
}

func publicSIM(sim device.SIM, available bool) SIMSnapshot {
	return SIMSnapshot{
		Available: available, Present: sim.Present, Ready: sim.Ready, PINStatus: sim.PINStatus,
		Operator: sim.Operator, MCC: sim.MCC, MNC: sim.MNC,
		IMSI: security.Mask(sim.IMSI, 5, 4), ICCID: security.Mask(sim.ICCID, 6, 4),
		Phone: maskPhone(sim.Phone), Registered: sim.Registered, Roaming: sim.Roaming,
	}
}

func signalSnapshot(signal device.Signal, available bool) SignalSnapshot {
	return SignalSnapshot{
		Available: available, RSSI: signal.RSSI, RSRP: signal.RSRP, RSRQ: signal.RSRQ,
		SINR: signal.SINR, Technology: signal.Technology, PLMN: signal.PLMN,
		Roaming: signal.Roaming, Registered: signal.Registered,
	}
}

func containsDevice(devices []device.Device, id string) bool {
	for _, d := range devices {
		if d.ID == id {
			return true
		}
	}
	return false
}

func maskPhone(value string) string {
	if value == "" {
		return ""
	}
	return security.Mask(value, 3, 2)
}

func truncate(value string, limit int) string {
	value = strings.ToValidUTF8(value, "")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
