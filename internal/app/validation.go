package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"qmi-web/internal/device"
)

// QMIValidation is a sanitized, ordered record of the SMS-only bring-up.
// It deliberately contains no IMEI, IMSI, ICCID, phone number, PDU, or SMS
// body. It is useful both in the real-test UI and in the final NAS report.
type QMIValidation struct {
	Mode              string     `json:"mode"`
	Stage             string     `json:"stage"`
	DeviceOpen        string     `json:"device_open"`
	DMS               string     `json:"dms"`
	UIM               string     `json:"uim"`
	NAS               string     `json:"nas"`
	WDS               string     `json:"wds"`
	WDSStatus         string     `json:"wds_status"`
	WMSList           string     `json:"wms_list"`
	WMSSubscribe      string     `json:"wms_subscribe"`
	WMSSetEventReport string     `json:"wms_set_event_report"`
	WMSIndication     string     `json:"wms_indication_register"`
	WMSBind           string     `json:"wms_bind_subscription"`
	SMS               string     `json:"sms"`
	ReadMessage       string     `json:"read_message"`
	Decoder           string     `json:"decoder"`
	SQLite            string     `json:"sqlite"`
	Dedup             string     `json:"dedup"`
	DataGuard         string     `json:"data_guard"`
	DeviceOwnership   string     `json:"device_ownership"`
	ReconnectCount    int        `json:"reconnect_count"`
	LastIndication    *time.Time `json:"last_wms_indication,omitempty"`
	LastReconcile     *time.Time `json:"last_storage_reconciliation,omitempty"`
	Stored            int        `json:"stored_messages"`
	Imported          int        `json:"imported_messages"`
	Terminal          bool       `json:"terminal"`
	WindowSeconds     int        `json:"window_seconds,omitempty"`
	RemainingSeconds  int        `json:"remaining_seconds,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	Detail            string     `json:"detail,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func newQMIValidation(real bool, window time.Duration) QMIValidation {
	mode, guard := "standard", "not-applicable"
	if real {
		mode, guard = "real-sms-only", "pending"
	}
	now := time.Now().UTC()
	validation := QMIValidation{
		Mode: mode, Stage: "idle", DeviceOpen: "pending", DMS: "pending",
		UIM: "pending", NAS: "pending", WDS: "pending", WMSList: "pending",
		WMSSubscribe: "pending", WMSSetEventReport: "pending", WMSIndication: "pending",
		WMSBind: "not-available", SMS: "not-requested", ReadMessage: "pending",
		Decoder: "pending", SQLite: "pending", Dedup: "pending", DataGuard: guard,
		DeviceOwnership: "not-observable", UpdatedAt: now,
	}
	if real && window > 0 {
		deadline := now.Add(window)
		validation.WindowSeconds = int(window.Seconds())
		validation.RemainingSeconds = validation.WindowSeconds
		validation.StartedAt = &now
		validation.Deadline = &deadline
	}
	return validation
}

func withRemaining(validation QMIValidation) QMIValidation {
	if validation.Deadline == nil {
		return validation
	}
	remaining := int(time.Until(*validation.Deadline).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	validation.RemainingSeconds = remaining
	return validation
}

func (a *App) setValidation(stage, outcome, detail string) {
	a.mu.Lock()
	v := a.validation
	v.Stage, v.UpdatedAt = stage, time.Now().UTC()
	switch stage {
	case "device_open":
		v.DeviceOpen = outcome
	case "dms":
		v.DMS = outcome
	case "uim":
		v.UIM = outcome
	case "nas":
		v.NAS = outcome
	case "wds":
		v.WDS = outcome
	case "wms_list":
		v.WMSList = outcome
	case "wms_subscribe":
		v.WMSSubscribe = outcome
	case "sms":
		v.SMS = outcome
	}
	if detail != "" {
		v.Detail = detail
	} else if outcome == "pass" || outcome == "running" {
		// Do not leave an earlier transient wait reason visible after its gate
		// has advanced successfully.
		v.Detail = ""
	}
	if outcome == "fail" || outcome == "blocked" {
		v.Terminal = v.Terminal || a.Config.Device.RealValidation
	}
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) setWDSStatus(status string) {
	a.mu.Lock()
	v := a.validation
	v.WDSStatus, v.UpdatedAt = status, time.Now().UTC()
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) recordDataGuard(state, wdsStatus string) {
	if !a.Config.Device.RealValidation {
		return
	}
	a.mu.Lock()
	v := a.validation
	v.DataGuard, v.UpdatedAt = state, time.Now().UTC()
	if wdsStatus != "" {
		v.WDSStatus = wdsStatus
	}
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) recordSMSScan(stored, imported int) {
	a.mu.Lock()
	v := a.validation
	v.Stored += stored
	v.Imported += imported
	v.UpdatedAt = time.Now().UTC()
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) recordStorageReconciliation() {
	a.mu.Lock()
	v := a.validation
	now := time.Now().UTC()
	v.LastReconcile, v.UpdatedAt = &now, now
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) recordWMSIndication() {
	a.mu.Lock()
	v := a.validation
	now := time.Now().UTC()
	v.LastIndication, v.UpdatedAt = &now, now
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) recordReconnect() {
	a.mu.Lock()
	v := a.validation
	v.ReconnectCount++
	v.UpdatedAt = time.Now().UTC()
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) setDeviceOwnership(value string) {
	a.mu.Lock()
	v := a.validation
	v.DeviceOwnership, v.UpdatedAt = value, time.Now().UTC()
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) setWMSSubscription(status device.WMSSubscription) {
	a.mu.Lock()
	v := a.validation
	if status.EventReport {
		v.WMSSetEventReport = "pass"
	}
	if status.IndicationRegister {
		v.WMSIndication = "pass"
	}
	if status.BindSubscription != "" {
		v.WMSBind = status.BindSubscription
	}
	v.UpdatedAt = time.Now().UTC()
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) recordSMSPipeline(saved, observed bool) {
	if !a.Config.Device.RealValidation || !observed {
		return
	}
	a.mu.Lock()
	v := a.validation
	// Pipeline.Ingest succeeded for this observed record, so it was read,
	// decoded, and handled by SQLite whether it inserted a row or was rejected
	// as an existing record after a process restart.
	v.ReadMessage, v.Decoder, v.SQLite = "pass", "pass", "pass"
	if !saved {
		// An observed reconciliation that does not insert a row is direct
		// deduplication evidence, including after a process restart.  It does
		// not imply that this process received a new SMS.
		v.Dedup = "pass"
	}
	v.UpdatedAt = time.Now().UTC()
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) markSMSReceived() {
	a.mu.Lock()
	v := a.validation
	if v.Mode == "real-sms-only" {
		v.Stage, v.SMS = "sms", "received"
		v.UpdatedAt = time.Now().UTC()
	}
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
}

func (a *App) startRealValidationWindow(ctxDone <-chan struct{}) {
	if !a.Config.Device.RealValidation {
		return
	}
	window := a.Config.Device.RealValidationWindow.Value()
	if window <= 0 {
		return
	}
	go func() {
		timer := time.NewTimer(window)
		defer timer.Stop()
		select {
		case <-ctxDone:
			return
		case <-timer.C:
			a.completeRealValidationWindow()
		}
	}()
}

func (a *App) completeRealValidationWindow() {
	a.mu.Lock()
	v := a.validation
	if v.Terminal {
		a.mu.Unlock()
		return
	}
	v.Stage, v.Terminal, v.UpdatedAt = "complete", true, time.Now().UTC()
	if v.SMS == "waiting" {
		v.SMS, v.Detail = "pending", "REAL_SMS_PENDING"
	} else {
		v.Detail = "REAL_SMS_WINDOW_COMPLETE"
	}
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
	a.disconnect()
}

func (a *App) failRealValidationDataGuard() {
	a.mu.Lock()
	v := a.validation
	if v.Terminal {
		a.mu.Unlock()
		return
	}
	v.Stage, v.DataGuard, v.Detail, v.Terminal, v.UpdatedAt = "data_guard", "fail", "DATA_GUARD_FAIL", true, time.Now().UTC()
	a.validation = v
	snapshot := withRemaining(v)
	a.mu.Unlock()
	a.persistValidation(snapshot)
	a.disconnect()
}

func (a *App) refreshValidationWindow() {
	if !a.Config.Device.RealValidation {
		return
	}
	a.mu.RLock()
	snapshot := withRemaining(a.validation)
	a.mu.RUnlock()
	a.persistValidation(snapshot)
}

// persistValidation writes only the sanitized validation state. The host-side
// watchdog can inspect this file without authenticating to the WebUI or ever
// reading a real SMS database.
func (a *App) persistValidation(snapshot QMIValidation) {
	path := a.Config.Device.RealValidationStatusFile
	if !a.Config.Device.RealValidation || path == "" {
		return
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		a.logValidationStatusError(err)
		return
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		a.logValidationStatusError(err)
		return
	}
	temporary, err := os.CreateTemp(directory, ".validation-status-*")
	if err != nil {
		a.logValidationStatusError(err)
		return
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err == nil {
		_, err = temporary.Write(append(payload, '\n'))
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		a.logValidationStatusError(err)
		return
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		a.logValidationStatusError(err)
	}
}

func (a *App) logValidationStatusError(err error) {
	if a.Log != nil {
		a.Log.Warn("write real validation status failed", map[string]any{"error": err.Error()})
	}
}

// ValidationSnapshot returns a value copy so callers cannot mutate runtime
// state or observe unmasked modem identities.
func (a *App) ValidationSnapshot() QMIValidation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return withRemaining(a.validation)
}
