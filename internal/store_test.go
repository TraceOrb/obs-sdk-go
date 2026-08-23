package internal

import (
	"context"
	"testing"
)

func TestStepIsNoopWithoutStore(t *testing.T) {
	t.Parallel()

	Step(nil, "handler", nil, nil)
}

func TestStepRecordsEventsOnlyWithStore(t *testing.T) {
	t.Parallel()

	store := NewStore()
	duration := 4
	Step(store, "db.query", map[string]string{"table": "users"}, &StepOptions{
		Level:      EventLevelInfo,
		DurationMs: &duration,
	})

	if len(store.events) != 1 {
		t.Fatalf("got %d events", len(store.events))
	}
	event := store.events[0]
	if event.Seq != 0 || event.Name != "db.query" || event.Level != EventLevelInfo {
		t.Fatalf("got %#v", event)
	}
	if event.DurationMs == nil || *event.DurationMs != 4 {
		t.Fatalf("got duration %#v", event.DurationMs)
	}
	if event.Attrs["table"] != "users" {
		t.Fatalf("got attrs %#v", event.Attrs)
	}
	if event.Timestamp == "" {
		t.Fatal("missing timestamp")
	}
	if store.RequestID() == "" {
		t.Fatal("missing request id")
	}
}

func TestRedactMergesExtraKeysOnStore(t *testing.T) {
	t.Parallel()

	store := NewStore()
	Redact(store, []string{"Email"})
	Redact(store, []string{"cpf"})

	if len(store.redactKeys) != 2 || store.redactKeys[0] != "email" || store.redactKeys[1] != "cpf" {
		t.Fatalf("got %#v", store.redactKeys)
	}
}

func TestStepDefaultsToInfoWhenOptionsOmitted(t *testing.T) {
	t.Parallel()

	store := NewStore()
	Step(store, "handler", nil, nil)

	if store.events[0].Level != EventLevelInfo {
		t.Fatalf("got %s", store.events[0].Level)
	}
	if store.events[0].DurationMs != nil {
		t.Fatal("duration should be omitted")
	}
	if store.events[0].Attrs != nil {
		t.Fatal("attrs should be omitted")
	}
}

func TestStepStopsAfterPerRequestCap(t *testing.T) {
	t.Parallel()

	store := NewStore()
	for i := 0; i < maxEventsPerRequest+5; i++ {
		Step(store, "step", nil, nil)
	}

	if len(store.events) != maxEventsPerRequest {
		t.Fatalf("got %d events", len(store.events))
	}
}

func TestSetTagsMergesOntoExistingTags(t *testing.T) {
	t.Parallel()

	store := NewStore()
	SetTags(store, map[string]string{"city": "4"})
	SetTags(store, map[string]string{"tenant": "acme"})

	if store.tags["city"] != "4" || store.tags["tenant"] != "acme" {
		t.Fatalf("got %#v", store.tags)
	}
}

func TestSetErrorMessageWritesToStore(t *testing.T) {
	t.Parallel()

	store := NewStore()
	SetErrorMessage(store, "timeout")
	SetResponseBody(store, map[string]any{"ok": false})

	if store.errorMessage != "timeout" {
		t.Fatalf("got %s", store.errorMessage)
	}
	if store.responseBody == nil {
		t.Fatal("expected response body")
	}
}

func TestStoreRoundTripOnContext(t *testing.T) {
	t.Parallel()

	store := NewStore()
	ctx := WithStore(context.Background(), store)
	if StoreFrom(ctx) != store {
		t.Fatal("store mismatch")
	}
	if StoreFrom(context.Background()) != nil {
		t.Fatal("expected nil store")
	}
}
