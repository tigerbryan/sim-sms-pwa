// Package logging emits structured JSON while retaining a bounded, safe UI
// buffer. Callers must never pass SMS bodies or credentials as attributes.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"qmi-web/internal/events"
)

type Entry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type Recorder struct {
	logger *slog.Logger
	closer io.Closer
	level  *slog.LevelVar
	bus    *events.Bus
	mu     sync.RWMutex
	items  []Entry
}

func New(path, level string, bus *events.Bus) (*Recorder, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return nil, err
	}
	var slogLevel slog.Level
	if level == "debug" {
		slogLevel = slog.LevelDebug
	}
	levelVar := &slog.LevelVar{}
	levelVar.Set(slogLevel)
	handler := slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), &slog.HandlerOptions{Level: levelVar})
	return &Recorder{logger: slog.New(handler), closer: file, level: levelVar, bus: bus}, nil
}

func (r *Recorder) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	err := r.closer.Close()
	r.closer = nil
	return err
}

func (r *Recorder) SetLevel(level string) error {
	if r == nil || r.level == nil {
		return nil
	}
	switch level {
	case "debug":
		r.level.Set(slog.LevelDebug)
	case "info":
		r.level.Set(slog.LevelInfo)
	case "warn":
		r.level.Set(slog.LevelWarn)
	case "error":
		r.level.Set(slog.LevelError)
	default:
		return fmt.Errorf("unsupported log level %q", level)
	}
	return nil
}

func (r *Recorder) Debug(message string, fields map[string]any) {
	r.add(slog.LevelDebug, message, fields)
}
func (r *Recorder) Info(message string, fields map[string]any) {
	r.add(slog.LevelInfo, message, fields)
}
func (r *Recorder) Warn(message string, fields map[string]any) {
	r.add(slog.LevelWarn, message, fields)
}
func (r *Recorder) Error(message string, fields map[string]any) {
	r.add(slog.LevelError, message, fields)
}

func (r *Recorder) add(level slog.Level, message string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	r.logger.Log(context.Background(), level, message, mapToAttrs(fields)...)
	entry := Entry{Time: time.Now().UTC(), Level: level.String(), Message: message, Fields: fields}
	r.mu.Lock()
	r.items = append(r.items, entry)
	if len(r.items) > 500 {
		r.items = append([]Entry(nil), r.items[len(r.items)-500:]...)
	}
	bus := r.bus
	r.mu.Unlock()
	if bus != nil {
		bus.Publish(events.Event{Type: events.Log, Data: map[string]any{"level": entry.Level, "message": message, "fields": fields}})
	}
}

func (r *Recorder) Recent(limit int) []Entry {
	if limit < 1 || limit > 500 {
		limit = 500
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	start := len(r.items) - limit
	if start < 0 {
		start = 0
	}
	return append([]Entry(nil), r.items[start:]...)
}

func (r *Recorder) SetBus(bus *events.Bus) {
	r.mu.Lock()
	r.bus = bus
	r.mu.Unlock()
}

func mapToAttrs(fields map[string]any) []any {
	items := make([]any, 0, len(fields))
	for key, value := range fields {
		items = append(items, slog.Any(key, value))
	}
	return items
}
