package traceorb

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestObserveDoesNotBlockHandler(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	doer := &captureDoer{handler: func(call int) (int, error) {
		<-gate
		return 202, nil
	}}
	client, err := New(Options{
		IngestURL:       "http://obs.test/v1/ingest",
		WriteKey:        "ok_write_test_secret",
		Service:         "demo",
		Env:             "test",
		MaxQueue:        1,
		FlushIntervalMs: 0,
		HTTP:            doer,
		OnDrop:          func(DropReason) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	req = req.WithContext(ContextWithStore(req.Context()))

	done := make(chan struct{})
	go func() {
		client.ObserveHTTP(req, http.StatusOK, map[string]any{"ok": true}, nil, MiddlewareOptions{})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ObserveHTTP blocked on ingest")
	}

	close(gate)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		doer.mu.Lock()
		got := doer.calls
		doer.mu.Unlock()
		if got >= 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("want at least 1 call after release")
}

func TestObserveHTTPUsesStoreSnapshotNotLiveStore(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{status: 202}
	client, err := New(Options{
		IngestURL:       "http://obs.test/v1/ingest",
		WriteKey:        "ok_write_test_secret",
		Service:         "demo",
		Env:             "test",
		MaxQueue:        1,
		FlushIntervalMs: 0,
		HTTP:            doer,
		OnDrop:          func(DropReason) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	req = req.WithContext(ContextWithStore(req.Context()))
	client.Step(req.Context(), "before", nil, nil)
	RecordError(req, "first")

	client.ObserveHTTP(req, http.StatusInternalServerError, map[string]any{"ok": false}, nil, MiddlewareOptions{})

	RecordError(req, "second-after-observe")
	client.Step(req.Context(), "after", nil, nil)

	waitBodies(t, doer, 1)
	client.Flush()

	got := doer.bodies[0].Requests[0]
	if got.ErrorMessage != "first" {
		t.Fatalf("got errorMessage %q, want first (snapshot)", got.ErrorMessage)
	}
	if len(got.Events) != 2 {
		t.Fatalf("got %d events, want 2 from snapshot", len(got.Events))
	}
	names := []string{got.Events[0].Name, got.Events[1].Name}
	if names[0] != "before" || names[1] != "unhandled.error" {
		t.Fatalf("got events %#v", names)
	}
}
