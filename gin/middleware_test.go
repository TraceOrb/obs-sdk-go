package gin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	traceorb "github.com/TraceOrb/obs-sdk-go"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type captureDoer struct {
	mu     sync.Mutex
	bodies []traceorb.IngestPayload
}

func (c *captureDoer) Do(req *http.Request) (*http.Response, error) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var payload traceorb.IngestPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.bodies = append(c.bodies, payload)
	c.mu.Unlock()

	return &http.Response{
		StatusCode: 202,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func waitBodies(t *testing.T, doer *captureDoer, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		doer.mu.Lock()
		got := len(doer.bodies)
		doer.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("want %d bodies", n)
}

func flushAndWaitBodies(t *testing.T, client *traceorb.Client, doer *captureDoer, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		client.Flush()
		doer.mu.Lock()
		got := len(doer.bodies)
		doer.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("want %d bodies", n)
}

func testClient(t *testing.T, doer *captureDoer) *traceorb.Client {
	t.Helper()

	client, err := traceorb.New(traceorb.Options{
		IngestURL:       "http://obs.test/v1/ingest",
		WriteKey:        "ok_write_test_secret",
		Service:         "demo",
		Env:             "test",
		FlushIntervalMs: 0,
		HTTP:            doer,
		OnDrop:          func(traceorb.DropReason) {},
	})
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func TestGinMiddlewareRedactsAuthorizationAndCapturesRoute(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client := testClient(t, doer)
	defer client.Close()

	router := gin.New()
	router.Use(Middleware(client, Options{
		ResolveTags: func(c *gin.Context) map[string]string {
			return map[string]string{"city": "6"}
		},
	}))
	router.GET("/v1/orders/:id", func(c *gin.Context) {
		client.Step(c.Request.Context(), "handler", nil, nil)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/orders/9?limit=1", strings.NewReader(`{"password":"hunter2","sku":"abc"}`))
	req.Header.Set("Authorization", "Bearer super-secret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	flushAndWaitBodies(t, client, doer, 1)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}

	raw, _ := json.Marshal(doer.bodies[0])
	if strings.Contains(string(raw), "super-secret") || strings.Contains(string(raw), "hunter2") {
		t.Fatal("secret leaked")
	}

	request := doer.bodies[0].Requests[0]
	if request.RoutePattern != "/v1/orders/:id" {
		t.Fatalf("got route %s", request.RoutePattern)
	}
	if request.Tags["city"] != "6" {
		t.Fatalf("got tags %#v", request.Tags)
	}
	if len(request.Events) != 1 || request.Events[0].Name != "handler" {
		t.Fatalf("got events %#v", request.Events)
	}
}

func TestGinErrorHandlerRecordsWithoutChangingStatus(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	client := testClient(t, doer)
	defer client.Close()

	router := gin.New()
	router.Use(Middleware(client, Options{}))
	router.Use(ErrorHandler(client))
	router.GET("/fail", func(c *gin.Context) {
		_ = c.Error(errors.New("timeout"))
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	flushAndWaitBodies(t, client, doer, 1)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d", rec.Code)
	}

	request := doer.bodies[0].Requests[0]
	if request.ErrorMessage != "timeout" {
		t.Fatalf("got %s", request.ErrorMessage)
	}
	if request.Events[0].Name != "unhandled.error" {
		t.Fatalf("got %#v", request.Events)
	}
}
