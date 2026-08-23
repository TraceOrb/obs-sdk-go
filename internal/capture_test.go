package internal

import (
	"strconv"
	"testing"
	"time"
)

func TestIngestRequestFromCaptureMapsHTTPAndStore(t *testing.T) {
	t.Parallel()

	store := pastStore()
	store.events = append(store.events, IngestEvent{
		Seq:       0,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Name:      "handler",
		Level:     EventLevelInfo,
	})
	store.errorMessage = "boom"
	store.responseBody = map[string]any{"ok": true}
	store.tags = map[string]string{"city": "6"}

	request := IngestFromCapture(CapturedHTTP{
		method:       "POST",
		path:         "/v1/orders?limit=1",
		routePattern: "/v1/orders",
		statusCode:   201,
		query:        map[string]any{"limit": "1"},
		headers:      map[string]any{"Accept": "application/json"},
		body:         map[string]any{"sku": "abc"},
		ip:           "127.0.0.1",
		userAgent:    "vitest",
		extraTags:    map[string]string{"tenant": "acme"},
		userID:       "u-1",
	}, store, testMeta())

	if request.RequestID != store.requestID {
		t.Fatal("request id mismatch")
	}
	if request.Method != "POST" || request.Path != "/v1/orders?limit=1" || request.RoutePattern != "/v1/orders" {
		t.Fatalf("got %s %s %s", request.Method, request.Path, request.RoutePattern)
	}
	if request.StatusCode != 201 {
		t.Fatalf("got status %d", request.StatusCode)
	}
	if request.DurationMs < 40 {
		t.Fatalf("got duration %d", request.DurationMs)
	}
	if request.Tags["city"] != "6" || request.Tags["tenant"] != "acme" {
		t.Fatalf("got tags %#v", request.Tags)
	}
	if request.UserID != "u-1" || request.IP != "127.0.0.1" || request.UserAgent != "vitest" {
		t.Fatal("optional fields mismatch")
	}
	if request.ErrorMessage != "boom" {
		t.Fatalf("got error %s", request.ErrorMessage)
	}
	if request.QueryJSON == "" || request.RequestHeadersJSON == "" || request.RequestBodyJSON == "" {
		t.Fatal("expected json fields")
	}
	if request.ResponseBodyJSON != `{"ok":true}` {
		t.Fatalf("got response %s", request.ResponseBodyJSON)
	}
	if len(request.Events) != 1 {
		t.Fatalf("got %d events", len(request.Events))
	}
}

func TestIngestRequestOmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	store := NewStore()
	request := IngestFromCapture(CapturedHTTP{
		method:       "POST",
		path:         "/v1/orders",
		routePattern: "/v1/orders",
		statusCode:   201,
		headers:      map[string]any{"Accept": "application/json"},
	}, store, testMeta())

	if request.IP != "" || request.UserAgent != "" || request.UserID != "" {
		t.Fatal("expected omitted optional strings")
	}
	if request.Tags != nil || request.QueryJSON != "" || request.RequestBodyJSON != "" {
		t.Fatal("expected omitted empty collections")
	}
	if request.Events != nil || request.ErrorMessage != "" {
		t.Fatal("expected omitted error and events")
	}
}

func TestIngestRequestCapsTags(t *testing.T) {
	t.Parallel()

	store := NewStore()
	extra := map[string]string{}
	for i := 0; i < maxTagsPerRequest+4; i++ {
		extra["k"+strconv.Itoa(i)] = "v"
	}

	request := IngestFromCapture(CapturedHTTP{
		method:       "GET",
		path:         "/",
		routePattern: "/",
		statusCode:   200,
		headers:      map[string]any{},
		extraTags:    extra,
	}, store, testMeta())

	if len(request.Tags) != maxTagsPerRequest {
		t.Fatalf("got %d tags", len(request.Tags))
	}
}

func TestFutureStartedAtYieldsZeroDuration(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.startedAt = time.Now().UTC().Add(60 * time.Second)
	request := IngestFromCapture(CapturedHTTP{
		method:       "GET",
		path:         "/",
		routePattern: "/",
		statusCode:   200,
		headers:      map[string]any{},
	}, store, testMeta())

	if request.DurationMs != 0 {
		t.Fatalf("got duration %d", request.DurationMs)
	}
}
