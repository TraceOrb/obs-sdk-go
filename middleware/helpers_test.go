package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"

	traceorb "github.com/TraceOrb/obs-sdk-go"
)

var errDown = errors.New("down")

func testClient(t *testing.T, doer traceorb.HTTPDoer) *traceorb.Client {
	t.Helper()

	if doer == nil {
		doer = &captureDoer{status: 202}
	}

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

type captureDoer struct {
	mu     sync.Mutex
	bodies []traceorb.IngestPayload
	status int
	err    error
}

func (c *captureDoer) Do(req *http.Request) (*http.Response, error) {
	if c.err != nil {
		return nil, c.err
	}

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

	status := c.status
	if status == 0 {
		status = 202
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
	}, nil
}
