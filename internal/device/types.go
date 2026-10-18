// Package device defines modem abstractions shared by real and mock backends.
package device

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNoDevice   = errors.New("no compatible QMI device detected")
	ErrDeviceBusy = errors.New("device may be in use by another service")
)

type Device struct {
	ID               string   `json:"id"`
	ControlPath      string   `json:"control_path"`
	Driver           string   `json:"driver"`
	USBVID           string   `json:"usb_vid"`
	USBPID           string   `json:"usb_pid"`
	Manufacturer     string   `json:"manufacturer"`
	Product          string   `json:"product"`
	NetworkInterface string   `json:"network_interface"`
	SerialPorts      []string `json:"serial_ports"`
	SysfsPath        string   `json:"sysfs_path"`
	Status           string   `json:"status"`
	Busy             bool     `json:"busy"`
}

type Info struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Revision     string `json:"revision"`
}

type SIM struct {
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

type Signal struct {
	RSSI       string `json:"rssi"`
	RSRP       string `json:"rsrp"`
	RSRQ       string `json:"rsrq"`
	SINR       string `json:"sinr"`
	Technology string `json:"technology"`
	PLMN       string `json:"plmn"`
	Roaming    bool   `json:"roaming"`
	Registered bool   `json:"registered"`
}

type Registration struct {
	Registered bool
	Roaming    bool
	State      string
	Technology string
	MCC        string
	MNC        string
}

// PacketService is an observational snapshot of the WDS packet-service state.
// It cannot start, stop, or otherwise alter a data session.
type PacketService struct {
	State string `json:"state"`
}

type SMSRecord struct {
	StorageType  uint8
	StorageIndex uint32
	Tag          uint8
	Raw          []byte
	ReceivedAt   time.Time
}

type SMSNotice struct {
	StorageType  *uint8
	StorageIndex *uint32
}

// WMSSubscription records only subscription capabilities and outcomes. It
// deliberately excludes modem identities, PDU data, phone numbers, and SMS
// content so it is safe for diagnostics and real-validation status files.
type WMSSubscription struct {
	EventReport        bool
	IndicationRegister bool
	BindSubscription   string
}

// Modem intentionally exposes no data-connect, carrier-profile, routing, DNS, SIM-write,
// modem-reset, or SMS-send methods. QMI Web is receive-only by architecture.
type Modem interface {
	Info(context.Context) (Info, error)
	SIM(context.Context) (SIM, error)
	Signal(context.Context) (Signal, error)
	Registration(context.Context) (Registration, error)
	PacketService(context.Context) (PacketService, error)
	ListSMS(context.Context) ([]SMSRecord, error)
	ReadSMS(context.Context, uint8, uint32) (SMSRecord, error)
	SubscribeSMS(context.Context) (<-chan SMSNotice, WMSSubscription, error)
	Close() error
}

type Backend interface {
	Name() string
	Scan(context.Context) ([]Device, error)
	Open(context.Context, string) (Modem, error)
}

type DeviceManager interface {
	Scan(context.Context) ([]Device, error)
	Open(context.Context, string) (Modem, error)
	BackendName() string
}
