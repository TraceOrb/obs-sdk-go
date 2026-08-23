package traceorb

import (
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

	captured := internal.IngestFromRequest(
		r,
		status,
		responseBody,
		requestBody,
		store,
		c.captureMeta(),
		internal.ObserveOptions{
			ResolveTags:         opts.ResolveTags,
			ResolveUserID:       opts.ResolveUserID,
			ResolveRoutePattern: opts.ResolveRoutePattern,
			RedactKeys:          opts.RedactKeys,
			ResolveRedactKeys:   opts.ResolveRedactKeys,
		},
	)
	c.Enqueue(captured)
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
