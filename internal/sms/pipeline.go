// Package sms decodes PDU payloads and persists a deduplicated SMS inbox.
package sms

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/warthog618/sms"
	"github.com/warthog618/sms/encoding/tpdu"
	"qmi-web/internal/database"
	"qmi-web/internal/device"
	"qmi-web/internal/events"
)

// ConcatInfo is the small, project-owned representation needed for safe local
// multipart inbox reassembly. It intentionally contains no PDU body or modem
// identity.
type ConcatInfo struct {
	IsConcat bool
	Ref      int
	RefBits  int
	Total    int
	Seq      int
}

type Decoded struct {
	Sender    string
	Body      string
	Timestamp time.Time
	Encoding  string
	Concat    ConcatInfo
}

type Pipeline struct {
	Store      *database.Store
	Events     *events.Bus
	OnComplete func(context.Context, database.SMSMessage)
	Source     string
	ingestMu   sync.Mutex
}

func Decode(raw []byte) (Decoded, error) {
	body, err := decodeBodyMaybeHex(raw)
	if err != nil {
		return Decoded{}, err
	}
	candidates := [][]byte{body}
	if stripped, ok := stripSMSC(body); ok {
		candidates = append(candidates, stripped)
	}
	if rpTPDU, ok := extractRPData(body); ok {
		candidates = append(candidates, rpTPDU)
	}
	var lastErr error
	for _, candidate := range candidates {
		sender, text, timestamp, concat, err := decodeDeliverTPDU(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		if timestamp.IsZero() {
			timestamp = time.Now().UTC()
		}
		return Decoded{Sender: sender, Body: text, Timestamp: timestamp.UTC(), Encoding: detectEncoding(candidate), Concat: concat}, nil
	}
	return Decoded{}, fmt.Errorf("decode SMS PDU: %w", lastErr)
}

func decodeBodyMaybeHex(raw []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) == 0 {
		return nil, errors.New("empty SMS PDU")
	}
	if len(trimmed)%2 == 0 && isHex(trimmed) {
		decoded, err := hex.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("decode hexadecimal PDU: %w", err)
		}
		return decoded, nil
	}
	return append([]byte(nil), raw...), nil
}

func isHex(value string) bool {
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func decodeDeliverTPDU(raw []byte) (string, string, time.Time, ConcatInfo, error) {
	pdu, err := sms.Unmarshal(raw, sms.AsMT)
	if err != nil {
		return "", "", time.Time{}, ConcatInfo{}, err
	}
	if pdu.MTI() != tpdu.MtDeliver {
		return "", "", time.Time{}, ConcatInfo{}, errors.New("PDU is not SMS-DELIVER")
	}
	text, err := sms.Decode([]*tpdu.TPDU{pdu})
	if err != nil {
		return "", "", time.Time{}, ConcatInfo{}, err
	}
	return pdu.OA.Number(), string(text), pdu.SCTS.Time, concatInfo(pdu), nil
}

func concatInfo(pdu *tpdu.TPDU) ConcatInfo {
	total, seq, ref, ok := pdu.ConcatInfo()
	if !ok {
		return ConcatInfo{}
	}
	bits := 8
	if _, ok := pdu.UDH.IE(0x08); ok {
		bits = 16
	}
	return ConcatInfo{IsConcat: true, Ref: ref, RefBits: bits, Total: total, Seq: seq}
}

// extractRPData accepts the standard RP-DATA envelope layout and returns its
// embedded TPDU. It only parses byte boundaries; it does not alter any modem
// state or acknowledge a message.
func extractRPData(raw []byte) ([]byte, bool) {
	if len(raw) < 5 || raw[0]&0x07 != 0x01 {
		return nil, false
	}
	index := 2 // RP-MTI and RP-Message-Reference
	for range 2 {
		if index >= len(raw) {
			return nil, false
		}
		length := int(raw[index])
		index++
		if length < 0 || index+length > len(raw) {
			return nil, false
		}
		index += length
	}
	if index >= len(raw) {
		return nil, false
	}
	length := int(raw[index])
	index++
	if length == 0 || index+length != len(raw) {
		return nil, false
	}
	return append([]byte(nil), raw[index:]...), true
}

func stripSMSC(raw []byte) ([]byte, bool) {
	if len(raw) < 2 {
		return nil, false
	}
	length := int(raw[0])
	if length < 1 || 1+length >= len(raw) {
		return nil, false
	}
	return raw[1+length:], true
}

func (p *Pipeline) Ingest(ctx context.Context, deviceID string, record device.SMSRecord) (int64, bool, error) {
	if p.Store == nil {
		return 0, false, errors.New("SMS store is not configured")
	}
	decoded, err := Decode(record.Raw)
	if err != nil {
		return 0, false, err
	}
	return p.IngestDecoded(ctx, deviceID, record.StorageIndex, record.Raw, decoded)
}

func (p *Pipeline) IngestDecoded(ctx context.Context, deviceID string, storageIndex uint32, raw []byte, decoded Decoded) (int64, bool, error) {
	if p.Store == nil {
		return 0, false, errors.New("SMS store is not configured")
	}
	if decoded.Timestamp.IsZero() {
		decoded.Timestamp = time.Now().UTC()
	}
	if decoded.Encoding == "" {
		decoded.Encoding = "unknown"
	}
	rawHash := hashBytes([]byte(deviceID + "\x00" + string(raw)))
	source := p.messageSource()
	if decoded.Concat.IsConcat {
		c := decoded.Concat
		if c.Total < 1 || c.Total > 255 || c.Seq < 1 || c.Seq > c.Total || (c.RefBits != 8 && c.RefBits != 16) || c.Ref < 0 || c.Ref >= 1<<c.RefBits {
			return 0, false, errors.New("invalid multipart SMS header")
		}
		// Modem indications and periodic reconciliation may overlap. Serialize
		// reassembly so two scans cannot claim overlapping incomplete groups.
		p.ingestMu.Lock()
		defer p.ingestMu.Unlock()
		added, err := p.Store.AddSMSPart(ctx, deviceID, decoded.Sender, decoded.Concat.Ref, decoded.Concat.RefBits, decoded.Concat.Total, decoded.Concat.Seq, decoded.Timestamp, decoded.Body, rawHash)
		if err != nil {
			return 0, false, err
		}
		parts, err := p.Store.Parts(ctx, deviceID, decoded.Sender, decoded.Concat.Ref, decoded.Concat.RefBits, decoded.Concat.Total, decoded.Timestamp)
		if err != nil {
			return 0, false, err
		}
		if len(parts) != decoded.Concat.Total {
			return 0, added, nil
		}
		var bodies, identities []string
		containsIncoming := false
		firstStamp, lastStamp := parts[0].Timestamp, parts[0].Timestamp
		for i, part := range parts {
			// Duplicate or missing sequence numbers mean this reference is
			// ambiguous. Retain every part instead of manufacturing mixed text.
			if part.Sequence != i+1 {
				return 0, added, nil
			}
			containsIncoming = containsIncoming || part.RawHash == rawHash
			if part.Timestamp.Before(firstStamp) {
				firstStamp = part.Timestamp
			}
			if part.Timestamp.After(lastStamp) {
				lastStamp = part.Timestamp
			}
			bodies = append(bodies, part.Body)
			identities = append(identities, part.RawHash)
		}
		if !containsIncoming || lastStamp.Sub(firstStamp) > database.MultipartAssemblyWindow {
			return 0, added, nil
		}
		body := strings.Join(bodies, "")
		ref := decoded.Concat.Ref
		message := database.SMSMessage{
			DeviceID: deviceID, Sender: decoded.Sender, Timestamp: parts[0].Timestamp,
			ReceivedAt: time.Now().UTC(), Encoding: decoded.Encoding, Body: body,
			IsMultipart: true, ReferenceNumber: &ref, PartsTotal: decoded.Concat.Total,
			PartsReceived: len(parts), RawHash: hashBytes([]byte(strings.Join(identities, "\x00"))), Source: source,
		}
		id, saved, err := p.Store.CompleteMultipartSMS(ctx, message, parts, multipartHash(deviceID, decoded, body))
		if err != nil || !saved {
			return id, saved, err
		}
		message.ID = id
		p.publish(ctx, events.SMSReassembled, message)
		return id, true, nil
	}
	index := int64(storageIndex)
	message := database.SMSMessage{
		DeviceID: deviceID, StorageIndex: &index, Sender: decoded.Sender,
		Timestamp: decoded.Timestamp, ReceivedAt: time.Now().UTC(),
		Encoding: decoded.Encoding, Body: decoded.Body, RawHash: rawHash, Source: source,
	}
	id, saved, err := p.Store.InsertSMS(ctx, message)
	if err != nil || !saved {
		return id, saved, err
	}
	message.ID = id
	p.publish(ctx, events.SMSReceived, message)
	return id, true, nil
}

func (p *Pipeline) messageSource() string {
	if p.Source == "modem" || p.Source == "test_fixture" {
		return p.Source
	}
	return "test_fixture"
}

func (p *Pipeline) publish(ctx context.Context, typ events.Type, message database.SMSMessage) {
	if p.Events != nil {
		p.Events.Publish(events.Event{Type: typ, Data: map[string]any{"sms_id": message.ID, "device_id": message.DeviceID}})
	}
	p.Store.AddEvent(ctx, string(typ), `{"sms_id":`+strconv.FormatInt(message.ID, 10)+`}`)
	if p.OnComplete != nil {
		p.OnComplete(ctx, message)
	}
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func multipartHash(deviceID string, decoded Decoded, body string) string {
	input := deviceID + "\x00" + decoded.Sender + "\x00" + strconv.Itoa(decoded.Concat.Ref) + "\x00" + strconv.Itoa(decoded.Concat.RefBits) + "\x00" + strconv.Itoa(decoded.Concat.Total) + "\x00" + body
	return hashBytes([]byte(input))
}

func detectEncoding(tpdu []byte) string {
	// DELIVER TPDU: first octet, OA digits, TOA, OA bytes, PID, then DCS.
	if len(tpdu) < 5 {
		return "PDU"
	}
	oaDigits := int(tpdu[1])
	dcsIndex := 3 + (oaDigits+1)/2 + 1
	if dcsIndex >= len(tpdu) {
		return "PDU"
	}
	switch tpdu[dcsIndex] & 0x0C {
	case 0x00:
		return "GSM7"
	case 0x04:
		return "8-bit"
	case 0x08:
		return "UCS2"
	default:
		return "PDU"
	}
}
