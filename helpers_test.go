package traceorb

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
)

func testClient(t *testing.T, doer HTTPDoer) *Client {
	t.Helper()

	if doer == nil {
		doer = &captureDoer{status: 202}
	}

	client, err := New(Options{
		IngestURL:       "http://obs.test/v1/ingest",
		WriteKey:        "ok_write_test_secret",
		Service:         "demo",
		Env:             "test",
		FlushIntervalMs: 0,
		HTTP:            doer,
		OnDrop:          func(DropReason) {},
	})
	if err != nil {
		t.Fatal(err)
	}

	return client
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
