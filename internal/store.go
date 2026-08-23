package internal

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Store struct {
	requestID    string
	startedAt    time.Time
	events       []IngestEvent
	tags         map[string]string
	redactKeys   []string
	errorMessage string
	responseBody any
}

func NewStore() *Store {
	return &Store{
		requestID:  newRequestID(),
		startedAt:  time.Now().UTC(),
		events:     make([]IngestEvent, 0),
		tags:       map[string]string{},
		redactKeys: []string{},
	}
}

func (s *Store) RequestID() string {
	if s == nil {
		return ""
	}

	return s.requestID
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(b[:])
	return hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:32]
}

func Step(store *Store, name string, attrs map[string]string, options *StepOptions) {
	if store == nil {
		return
	}

	if len(store.events) >= maxEventsPerRequest {
		return
	}

	level := EventLevelInfo
	var duration *int
	if options != nil {
		if options.Level != "" {
			level = options.Level
		}

		duration = options.DurationMs
	}

	event := IngestEvent{
		Seq:        len(store.events),
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Name:       name,
		Level:      level,
		DurationMs: duration,
	}

	if attrs != nil {
		event.Attrs = attrs
	}

	store.events = append(store.events, event)
}

func SetTags(store *Store, tags map[string]string) {
	if store == nil {
		return
	}

	for key, value := range tags {
		store.tags[key] = value
	}
}

func Redact(store *Store, keys []string) {
	if store == nil {
		return
	}

	store.redactKeys = MergeRedactKeys([][]string{store.redactKeys, keys})
}

func SetErrorMessage(store *Store, message string) {
	if store == nil {
		return
	}

	store.errorMessage = message
}

func SetResponseBody(store *Store, body any) {
	if store == nil {
		return
	}

	store.responseBody = body
}
