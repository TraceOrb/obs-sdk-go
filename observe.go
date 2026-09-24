package traceorb

import (
	"math/rand"
	"net/http"

	"github.com/TraceOrb/obs-sdk-go/internal"
)

type MiddlewareOptions struct {
	Skip                func(r *http.Request) bool
	ResolveTags         func(r *http.Request) map[string]string
	ResolveUserID       func(r *http.Request) string
	ResolveRoutePattern func(r *http.Request) string
	RedactKeys          []string
	ResolveRedactKeys   func(r *http.Request) []string
}

func (c *Client) ObserveHTTP(r *http.Request, status int, responseBody any, requestBody any, opts MiddlewareOptions) {
	defer func() {
		_ = recover()
	}()

	store := internal.StoreFrom(r.Context())
	if store == nil {
		return
	}

	if status == 0 {
		status = http.StatusOK
	}

	observeOpts := internal.ObserveOptions{
		ResolveTags:         opts.ResolveTags,
		ResolveUserID:       opts.ResolveUserID,
		ResolveRoutePattern: opts.ResolveRoutePattern,
		RedactKeys:          opts.RedactKeys,
		ResolveRedactKeys:   opts.ResolveRedactKeys,
	}
	routePattern := internal.ResolveRoutePattern(r, observeOpts)
	resolved := c.policyForRoute(routePattern)

	random := 0.0
	if resolved.SampleRate > 0 && resolved.SampleRate < 1 {
		random = rand.Float64()
	}
	if !ShouldSample(resolved.SampleRate, status, random) {
		return
	}

	snap, respBody := internal.SnapshotFromRequest(r, status, responseBody, requestBody, observeOpts)
	meta := c.captureMeta(resolved.Capture)
	storeSnap := internal.PrepareIngestStore(store, respBody)

	go func() {
		defer func() {
			_ = recover()
		}()

		captured := internal.IngestFromCapture(snap, storeSnap, meta)
		c.Enqueue(captured)
	}()
}

func RecordError(r *http.Request, err any) {
	internal.RecordUnhandledError(internal.StoreFrom(r.Context()), err)
}

func ReadAndRestoreBody(r *http.Request, maxBytes int) any {
	return internal.ReadAndRestoreBody(r, maxBytes)
}

func DecodeCapturedBody(raw []byte) any {
	return internal.DecodeCapturedBody(raw)
}
