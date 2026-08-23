package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	traceorb "github.com/TraceOrb/obs-sdk-go"
)

func TestIngestDownStillServesTheRequest(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{err: errDown}
	client := testClient(t, doer)
	defer client.Close()

	handler := Middleware(client, Options{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}

	client.Flush()
}

func TestAuthorizationNeverAppearsInPostedJSON(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client := testClient(t, doer)
	defer client.Close()

	handler := Middleware(client, Options{
		ResolveTags: func(r *http.Request) map[string]string {
			return map[string]string{"city": "6"}
		},
		ResolveRoutePattern: func(r *http.Request) string {
			return "/v1/orders"
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=1", strings.NewReader(`{"password":"hunter2","sku":"abc"}`))
	req.Header.Set("Authorization", "Bearer super-secret")
	req.Header.Set("User-Agent", "vitest")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	client.Flush()

	if len(doer.bodies) != 1 {
		t.Fatalf("got %d bodies", len(doer.bodies))
	}

	raw, err := json.Marshal(doer.bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	posted := string(raw)
	if strings.Contains(posted, "super-secret") {
		t.Fatal("secret leaked")
	}
	if strings.Contains(posted, "hunter2") {
		t.Fatal("password leaked")
	}

	request := doer.bodies[0].Requests[0]
	if request.Service != "demo" || request.Env != "test" {
		t.Fatalf("got service/env %s %s", request.Service, request.Env)
	}
	if request.RoutePattern != "/v1/orders" {
		t.Fatalf("got route %s", request.RoutePattern)
	}
	if request.Tags["city"] != "6" {
		t.Fatalf("got tags %#v", request.Tags)
	}
	if !strings.Contains(request.RequestHeadersJSON, traceorb.Redacted) {
		t.Fatalf("headers %s", request.RequestHeadersJSON)
	}
	if !strings.Contains(request.RequestBodyJSON, traceorb.Redacted) {
		t.Fatalf("body %s", request.RequestBodyJSON)
	}
	if !strings.Contains(request.RequestBodyJSON, "abc") {
		t.Fatalf("sku missing in %s", request.RequestBodyJSON)
	}
	if request.ResponseBodyJSON != `{"ok":true}` {
		t.Fatalf("got response %s", request.ResponseBodyJSON)
	}
}

func TestRedactKeysFromClientMiddlewareAndRequest(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client, err := traceorb.New(traceorb.Options{
		IngestURL:       "http://obs.test/v1/ingest",
		WriteKey:        "ok_write_test_secret",
		Service:         "demo",
		Env:             "test",
		FlushIntervalMs: 0,
		HTTP:            doer,
		OnDrop:          func(traceorb.DropReason) {},
		RedactKeys:      []string{"email"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	handler := Middleware(client, Options{
		RedactKeys: []string{"cpf"},
		ResolveRedactKeys: func(r *http.Request) []string {
			return []string{r.Header.Get("X-Redact")}
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client.Redact(r.Context(), []string{"ssn"})
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(`{"email":"ada@example.com","cpf":"123","phone":"999","ssn":"000","sku":"abc"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redact", "phone")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	client.Flush()

	body := doer.bodies[0].Requests[0].RequestBodyJSON
	if strings.Contains(body, "ada@example.com") {
		t.Fatal("email leaked")
	}
	if strings.Contains(body, `"123"`) {
		t.Fatal("cpf leaked")
	}
	if strings.Contains(body, "999") {
		t.Fatal("phone leaked")
	}
	if strings.Contains(body, "000") {
		t.Fatal("ssn leaked")
	}
	if !strings.Contains(body, "abc") {
		t.Fatal("sku should remain")
	}
	if !strings.Contains(body, traceorb.Redacted) {
		t.Fatal("expected redacted markers")
	}
}

func TestStepInsideBoundStoreIsIncludedInBatch(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client := testClient(t, doer)
	defer client.Close()

	handler := Middleware(client, Options{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client.Step(r.Context(), "handler", nil, nil)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	client.Flush()

	events := doer.bodies[0].Requests[0].Events
	if len(events) != 1 || events[0].Name != "handler" || events[0].Seq != 0 || events[0].Level != traceorb.EventLevelInfo {
		t.Fatalf("got %#v", events)
	}
}

func TestResolverPanicIsSwallowed(t *testing.T) {
	t.Parallel()

	client := testClient(t, &captureDoer{status: 202})
	defer client.Close()

	handler := Middleware(client, Options{
		ResolveTags: func(r *http.Request) map[string]string {
			panic("boom")
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
}
