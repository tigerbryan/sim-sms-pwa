//go:build linux

package qmi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	qmilibrary "github.com/iniwex5/qmi-go/pkg/qmi"
	"qmi-web/internal/device"
)

// Open allocates DMS, UIM, NAS, one read-only WDS status client, and WMS.
// No method in this adapter starts, stops, configures, or otherwise changes a
// packet-data session.
func (b *Backend) Open(ctx context.Context, id string) (device.Modem, error) {
	devices, err := b.Scan(ctx)
	if err != nil {
		return nil, err
	}
	d, err := device.FindByID(devices, id)
	if err != nil {
		return nil, err
	}
	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	opts := qmilibrary.DefaultClientOptions()
	opts.DefaultRequestTimeout = 5 * time.Second
	opts.ReadDeadline = 15 * time.Second
	opts.UseProxy = false
	opts.UseQRTR = false
	client, err := qmilibrary.NewClientWithOptions(openCtx, d.ControlPath, opts)
	if err != nil {
		if errors.Is(err, syscall.EBUSY) || strings.Contains(strings.ToLower(err.Error()), "busy") {
			d.Busy, d.Status = true, "busy"
			return nil, fmt.Errorf("%w: %s", device.ErrDeviceBusy, d.ControlPath)
		}
		return nil, fmt.Errorf("open QMI control node: %w", err)
	}
	m := &modem{device: d, client: client}
	if m.dms, err = qmilibrary.NewDMSServiceWithContext(openCtx, client); err != nil {
		_ = m.Close()
		return nil, fmt.Errorf("allocate DMS: %w", err)
	}
	if m.uim, err = qmilibrary.NewUIMServiceWithContext(openCtx, client); err != nil {
		_ = m.Close()
		return nil, fmt.Errorf("allocate UIM: %w", err)
	}
	if m.nas, err = qmilibrary.NewNASServiceWithContext(openCtx, client); err != nil {
		_ = m.Close()
		return nil, fmt.Errorf("allocate NAS: %w", err)
	}
	if m.wds, err = qmilibrary.NewWDSServiceWithContext(openCtx, client); err != nil {
		_ = m.Close()
		return nil, fmt.Errorf("allocate WDS status client: %w", err)
	}
	if m.wms, err = qmilibrary.NewWMSServiceWithContext(openCtx, client); err != nil {
		_ = m.Close()
		return nil, fmt.Errorf("allocate WMS: %w", err)
	}
	return m, nil
}

type modem struct {
	device device.Device
	client *qmilibrary.Client
	dms    *qmilibrary.DMSService
	nas    *qmilibrary.NASService
	uim    *qmilibrary.UIMService
	wms    *qmilibrary.WMSService
	wds    *qmilibrary.WDSService
	once   sync.Once
}

func (m *modem) Info(ctx context.Context) (device.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	manufacturer, err := m.dms.GetManufacturer(ctx)
	if err != nil {
		return device.Info{}, err
	}
	model, err := m.dms.GetModel(ctx)
	if err != nil {
		return device.Info{}, err
	}
	revision, _, err := m.dms.GetDeviceRevision(ctx)
	if err != nil {
		return device.Info{}, err
	}
	return device.Info{Manufacturer: manufacturer, Model: model, Revision: revision}, nil
}

func (m *modem) SIM(ctx context.Context) (device.SIM, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	status, err := m.uim.GetCardStatus(ctx)
	if err != nil {
		return device.SIM{}, err
	}
	statusText := strings.ToLower(fmt.Sprint(status))
	imsi, _ := m.uim.GetIMSI(ctx)
	iccid, _ := m.uim.GetICCID(ctx)
	return device.SIM{
		Present:   !strings.Contains(statusText, "absent"),
		Ready:     strings.Contains(statusText, "ready") || strings.Contains(statusText, "available"),
		PINStatus: fmt.Sprint(status), IMSI: imsi, ICCID: iccid,
	}, nil
}

func (m *modem) Registration(ctx context.Context) (device.Registration, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	serving, err := m.nas.GetServingSystem(ctx)
	if err != nil {
		return device.Registration{}, err
	}
	state := strings.ToLower(fmt.Sprint(serving.RegistrationState))
	registered := state == "registered" || state == "roaming"
	roaming := strings.Contains(state, "roaming")
	return device.Registration{
		Registered: registered, Roaming: roaming, State: state,
		Technology: technologyName(serving.RadioInterface),
		MCC:        fmt.Sprintf("%03d", serving.MCC), MNC: fmt.Sprintf("%02d", serving.MNC),
	}, nil
}

func (m *modem) PacketService(ctx context.Context) (device.PacketService, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	status, err := m.wds.GetPacketServiceStatus(ctx)
	if err != nil {
		return device.PacketService{}, err
	}
	return device.PacketService{State: status.String()}, nil
}

func (m *modem) Signal(ctx context.Context) (device.Signal, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	raw, err := m.nas.GetSignalStrength(ctx)
	if err != nil {
		return device.Signal{}, err
	}
	registration, _ := m.Registration(ctx)
	return device.Signal{
		RSSI:       signalValue(int(raw.RSSI), "dBm"),
		RSRP:       signalValue(int(raw.RSRP), "dBm"),
		RSRQ:       signalValue(int(raw.RSRQ), "dB"),
		SINR:       signalTenthValue(int(raw.SNR)),
		Technology: registration.Technology, PLMN: registration.MCC + registration.MNC,
		Roaming: registration.Roaming, Registered: registration.Registered,
	}, nil
}

func (m *modem) ListSMS(ctx context.Context) ([]device.SMSRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result := make([]device.SMSRecord, 0)
	seen := make(map[string]struct{})
	var lastErr error
	listed := false
	// QMI storage 0 (UIM/SM) and 1 (NV/ME) are probed independently for both
	// MT tags. ListMessagesAuto returns after the first successful tag probe,
	// even when that result is empty, and can therefore miss MT-read records.
	// This explicit loop is receive-only and never modifies a message tag.
	for _, storage := range []uint8{0, 1} {
		for _, tag := range []qmilibrary.MessageTagType{qmilibrary.TagTypeMTNotRead, qmilibrary.TagTypeMTRead} {
			items, err := m.wms.ListMessages(ctx, storage, tag)
			if err != nil {
				lastErr = err
				continue
			}
			listed = true
			for _, item := range items {
				key := fmt.Sprintf("%d/%d", storage, item.Index)
				if _, ok := seen[key]; ok {
					continue
				}
				raw, err := m.wms.RawReadMessage(ctx, storage, item.Index)
				if err != nil {
					lastErr = err
					continue
				}
				seen[key] = struct{}{}
				result = append(result, device.SMSRecord{StorageType: storage, StorageIndex: item.Index, Tag: uint8(item.Tag), Raw: raw, ReceivedAt: time.Now().UTC()})
			}
		}
	}
	if !listed && lastErr != nil {
		return nil, lastErr
	}
	return result, nil
}

func (m *modem) ReadSMS(ctx context.Context, storage uint8, index uint32) (device.SMSRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	raw, err := m.wms.RawReadMessage(ctx, storage, index)
	if err != nil {
		return device.SMSRecord{}, err
	}
	return device.SMSRecord{StorageType: storage, StorageIndex: index, Raw: raw, ReceivedAt: time.Now().UTC()}, nil
}

func (m *modem) SubscribeSMS(ctx context.Context) (<-chan device.SMSNotice, device.WMSSubscription, error) {
	status := device.WMSSubscription{BindSubscription: "not_available_in_qmi_go_v0.6.4"}
	registerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := m.wms.RegisterEventReport(registerCtx); err != nil {
		return nil, status, err
	}
	status.EventReport = true
	if err := m.wms.IndicationRegister(registerCtx, false); err != nil {
		return nil, status, err
	}
	status.IndicationRegister = true
	out := make(chan device.SMSNotice, 16)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-m.client.Events():
				if !ok {
					return
				}
				if event.Type == qmilibrary.EventNewMessage {
					// The upstream event does not expose an index in a stable
					// public API. The application re-scans storage and uses its
					// database uniqueness constraint to avoid duplicate saves.
					select {
					case out <- device.SMSNotice{}:
					default:
					}
				}
			}
		}
	}()
	return out, status, nil
}

func (m *modem) Close() error {
	var result error
	m.once.Do(func() {
		for _, closeFn := range []func() error{
			func() error {
				if m.wds != nil {
					return m.wds.Close()
				}
				return nil
			},
			func() error {
				if m.wms != nil {
					return m.wms.Close()
				}
				return nil
			},
			func() error {
				if m.uim != nil {
					return m.uim.Close()
				}
				return nil
			},
			func() error {
				if m.nas != nil {
					return m.nas.Close()
				}
				return nil
			},
			func() error {
				if m.dms != nil {
					return m.dms.Close()
				}
				return nil
			},
			func() error {
				if m.client != nil {
					return m.client.Close()
				}
				return nil
			},
		} {
			if err := closeFn(); err != nil && result == nil {
				result = err
			}
		}
	})
	return result
}

func technologyName(value uint8) string {
	switch value {
	case 4:
		return "GSM"
	case 5:
		return "UMTS"
	case 8:
		return "LTE"
	case 10:
		return "NR5G"
	default:
		return "N/A"
	}
}

func signalValue(value int, unit string) string {
	if value == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%d %s", value, unit)
}

func signalTenthValue(value int) string {
	if value == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.1f dB", float64(value)/10)
}
