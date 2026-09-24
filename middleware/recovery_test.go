package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	traceorb "github.com/TraceOrb/obs-sdk-go"
)

func TestRecordErrorWithoutStoreDoesNotPanic(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	RecordError(req, errors.New("boom"))
}

func TestRecoveryRecordsUnhandledErrorThenRepanics(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client := testClient(t, doer)
	defer client.Close()

	inner := Middleware(client, Options{})(Recovery(client)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("timeout"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	rec := httptest.NewRecorder()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected re-panic")
		}

		flushAndWaitBodies(t, client, doer, 1)

		request := doer.bodies[0].Requests[0]
		if request.ErrorMessage != "timeout" {
			t.Fatalf("got error %s", request.ErrorMessage)
		}
		if len(request.Events) == 0 || request.Events[0].Name != "unhandled.error" {
			t.Fatalf("got events %#v", request.Events)
		}
		if request.Events[0].Level != traceorb.EventLevelError {
			t.Fatalf("got level %s", request.Events[0].Level)
		}
	}()

	inner.ServeHTTP(rec, req)
}

func TestRecordErrorDoesNotOverwriteSetErrorMessage(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client := testClient(t, doer)
	defer client.Close()

	handler := Middleware(client, Options{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client.SetErrorMessage(r.Context(), "from-app")
		RecordError(r, errors.New("other"))
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	flushAndWaitBodies(t, client, doer, 1)

	request := doer.bodies[0].Requests[0]
	if request.ErrorMessage != "from-app" {
		t.Fatalf("got %s", request.ErrorMessage)
	}
	if request.Events[0].Name != "unhandled.error" {
		t.Fatalf("got %#v", request.Events)
	}
}
