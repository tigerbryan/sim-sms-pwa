// Package events provides a small in-process, non-blocking event bus.
package events

import (
	"sync"
	"time"
)

type Type string

const (
	DeviceConnected        Type = "DeviceConnected"
	DeviceDisconnected     Type = "DeviceDisconnected"
	SIMReady               Type = "SIMReady"
	SIMError               Type = "SIMError"
	RegistrationChanged    Type = "RegistrationChanged"
	SignalChanged          Type = "SignalChanged"
	SMSReceived            Type = "SMSReceived"
	SMSReassembled         Type = "SMSReassembled"
	NotificationFailed     Type = "NotificationFailed"
	DataConnectionDetected Type = "DataConnectionDetected"
	SecurityWarning        Type = "SecurityWarning"
	Log                    Type = "Log"
)

type Event struct {
	Type Type           `json:"type"`
	At   time.Time      `json:"at"`
	Data map[string]any `json:"data,omitempty"`
}

type Bus struct {
	mu   sync.RWMutex
	next uint64
	subs map[uint64]chan Event
}

func New() *Bus {
	return &Bus{subs: make(map[uint64]chan Event)}
}

func (b *Bus) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 1
	}
	b.mu.Lock()
	id := b.next
	b.next++
	ch := make(chan Event, buffer)
	b.subs[id] = ch
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if current, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(current)
		}
		b.mu.Unlock()
	}
}

func (b *Bus) Publish(e Event) {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
			// A slow Web client must not block a modem callback.
		}
	}
}
