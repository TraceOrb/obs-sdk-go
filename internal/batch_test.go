package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

func makeRequest(path string) IngestRequest {
	return IngestRequest{
		RequestID:    "req-" + path,
		Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		Method:       "GET",
		Path:         path,
		RoutePattern: path,
		StatusCode:   200,
		DurationMs:   3,
		Service:      "demo",
		Env:          "test",
	}
}

type captureDoer struct {
	mu      sync.Mutex
	bodies  []IngestPayload
	status  int
	err     error
	calls   int
	handler func(call int) (int, error)
}

func (c *captureDoer) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.calls++
	call := c.calls
	c.mu.Unlock()

	if c.handler != nil {
		status, err := c.handler(call)
		if err != nil {
			return nil, err
		}

		return jsonHTTPResponse(status), nil
	}

	if c.err != nil {
		return nil, c.err
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var payload IngestPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.bodies = append(c.bodies, payload)
	c.mu.Unlock()

	status := c.status
	if status == 0 {
		status = 202
	}

	return jsonHTTPResponse(status), nil
}

func jsonHTTPResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
	}
}

func testBatch(doer HTTPDoer, maxQueue int, flushSize int, onDrop OnDropFunc) *Batch {
	if onDrop == nil {
		onDrop = func(DropReason) {}
	}

	return NewBatch(BatchOptions{
		IngestURL:     "http://obs.test/v1/ingest",
		WriteKey:      "ok_write_test_secret",
		MaxQueue:      maxQueue,
		FlushSize:     flushSize,
		FlushInterval: 0,
		RetryDelays:   []time.Duration{0, 0},
		FetchTimeout:  50 * time.Millisecond,
		HTTP:          doer,
		OnDrop:        onDrop,
	})
}

func TestFlushPostsIngestContractShape(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	batch := testBatch(doer, 8, 100, nil)
	request := makeRequest("/health")
	batch.Enqueue(request)
	batch.Flush()

	if len(doer.bodies) != 1 {
		t.Fatalf("got %d bodies", len(doer.bodies))
	}
	if len(doer.bodies[0].Requests) != 1 {
		t.Fatalf("got %d requests", len(doer.bodies[0].Requests))
	}
	if doer.bodies[0].Requests[0].Path != "/health" {
		t.Fatalf("got path %s", doer.bodies[0].Requests[0].Path)
	}
}

func TestQueueNeverGrowsPastMaxQueue(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	batch := testBatch(doer, 2, 100, nil)
	batch.Enqueue(makeRequest("/a"))
	batch.Enqueue(makeRequest("/b"))
	batch.Enqueue(makeRequest("/c"))
	batch.Enqueue(makeRequest("/d"))

	if batch.Size() != 2 {
		t.Fatalf("got size %d", batch.Size())
	}

	batch.Flush()
	if len(doer.bodies) != 1 {
		t.Fatalf("got %d bodies", len(doer.bodies))
	}

	got := []string{
		doer.bodies[0].Requests[0].Path,
		doer.bodies[0].Requests[1].Path,
	}
	if got[0] != "/c" || got[1] != "/d" {
		t.Fatalf("got %#v", got)
	}
}

func TestNetworkErrorsDropAfterRetriesWithoutThrowing(t *testing.T) {
	t.Parallel()

	var drops []DropReason
	doer := &captureDoer{err: errors.New("down")}
	batch := testBatch(doer, 8, 100, func(reason DropReason) {
		drops = append(drops, reason)
	})
	batch.Enqueue(makeRequest("/x"))
	batch.Flush()

	if len(drops) != 1 || drops[0] != DropRetryExhausted {
		t.Fatalf("got drops %#v", drops)
	}
}

func Test401DropsWithoutRetrying(t *testing.T) {
	t.Parallel()

	var drops []DropReason
	doer := &captureDoer{status: 401}
	batch := testBatch(doer, 8, 100, func(reason DropReason) {
		drops = append(drops, reason)
	})
	batch.Enqueue(makeRequest("/x"))
	batch.Flush()

	if doer.calls != 1 {
		t.Fatalf("got %d calls", doer.calls)
	}
	if len(drops) != 1 || drops[0] != DropInvalidResponse {
		t.Fatalf("got drops %#v", drops)
	}
}

func Test200IsSuccess(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{status: 200}
	batch := testBatch(doer, 8, 100, nil)
	batch.Enqueue(makeRequest("/x"))
	batch.Flush()
}

func Test429RetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{handler: func(call int) (int, error) {
		if call == 1 {
			return 429, nil
		}

		return 202, nil
	}}
	batch := testBatch(doer, 8, 100, nil)
	batch.Enqueue(makeRequest("/x"))
	batch.Flush()

	if doer.calls != 2 {
		t.Fatalf("got %d calls", doer.calls)
	}
}

func TestUnknown4xxDropsWithoutRetrying(t *testing.T) {
	t.Parallel()

	var drops []DropReason
	doer := &captureDoer{status: 418}
	batch := testBatch(doer, 8, 100, func(reason DropReason) {
		drops = append(drops, reason)
	})
	batch.Enqueue(makeRequest("/x"))
	batch.Flush()

	if doer.calls != 1 {
		t.Fatalf("got %d calls", doer.calls)
	}
	if len(drops) != 1 || drops[0] != DropInvalidResponse {
		t.Fatalf("got drops %#v", drops)
	}
}

func Test500RetriesUntilExhausted(t *testing.T) {
	t.Parallel()

	var drops []DropReason
	doer := &captureDoer{status: 500}
	batch := testBatch(doer, 8, 100, func(reason DropReason) {
		drops = append(drops, reason)
	})
	batch.Enqueue(makeRequest("/x"))
	batch.Flush()

	if doer.calls != 3 {
		t.Fatalf("got %d calls", doer.calls)
	}
	if len(drops) != 1 || drops[0] != DropRetryExhausted {
		t.Fatalf("got drops %#v", drops)
	}
}

func TestFlushOfEmptyQueueDoesNotPost(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	batch := testBatch(doer, 8, 100, nil)
	batch.Flush()

	if doer.calls != 0 {
		t.Fatalf("got %d calls", doer.calls)
	}
}

func TestCloseIsSafeBeforeTimerExists(t *testing.T) {
	t.Parallel()

	doer := &captureDoer{}
	batch := testBatch(doer, 8, 100, nil)
	batch.Close()
	batch.Close()
}

func TestDefaultFlushSizeNeverExceedsIngestBatchCap(t *testing.T) {
	t.Parallel()

	if DefaultFlushSize(8) != 8 {
		t.Fatal("small queue should equal itself")
	}
	if DefaultFlushSize(10_000) != maxIngestBatch {
		t.Fatal("large queue should cap at ingest batch")
	}
}
