package traceorb

import (
	"testing"
)

func TestCreateClientAppliesDefaults(t *testing.T) {
	t.Parallel()

	client := testClient(t, nil)
	defer client.Close()

	if client.Service() != "demo" {
		t.Fatalf("got service %s", client.Service())
	}
	if client.Env() != "test" {
		t.Fatalf("got env %s", client.Env())
	}
	if client.MaxBodyBytes() != MaxBodyBytes {
		t.Fatalf("got max body %d", client.MaxBodyBytes())
	}
	if len(client.RedactKeys()) != 0 {
		t.Fatalf("got redact keys %#v", client.RedactKeys())
	}
}

func TestCreateClientKeepsCustomMaxBodyAndRedactKeys(t *testing.T) {
	t.Parallel()

	client, err := New(Options{
		IngestURL:       "http://obs.test/v1/ingest",
		WriteKey:        "ok_write_test_secret",
		Service:         "demo",
		Env:             "test",
		MaxBodyBytes:    512,
		FlushIntervalMs: 0,
		HTTP:            &captureDoer{status: 202},
		RedactKeys:      []string{"Email", "cpf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if client.MaxBodyBytes() != 512 {
		t.Fatalf("got %d", client.MaxBodyBytes())
	}
	got := client.RedactKeys()
	if len(got) != 2 || got[0] != "email" || got[1] != "cpf" {
		t.Fatalf("got %#v", got)
	}
}

func TestClientEnqueueFlushAndClose(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client := testClient(t, doer)
	defer client.Close()

	client.Enqueue(IngestRequest{
		RequestID:    "req-1",
		Timestamp:    "2026-01-01T00:00:00Z",
		Method:       "GET",
		Path:         "/health",
		RoutePattern: "/health",
		StatusCode:   200,
		DurationMs:   1,
		Service:      "demo",
		Env:          "test",
	})
	client.Flush()

	if len(doer.bodies) != 1 {
		t.Fatalf("got %d bodies", len(doer.bodies))
	}
	if doer.bodies[0].Requests[0].RequestID != "req-1" {
		t.Fatalf("got %s", doer.bodies[0].Requests[0].RequestID)
	}
}

func TestNewRequiresIngestURLWriteKeyServiceEnv(t *testing.T) {
	t.Parallel()

	_, err := New(Options{})
	if err == nil {
		t.Fatal("expected error")
	}
}
