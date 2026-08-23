package middleware

import (
	"net/http"

	traceorb "github.com/TraceOrb/obs-sdk-go"
)

type Options = traceorb.MiddlewareOptions

func Middleware(client *traceorb.Client, opts Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if opts.Skip != nil && opts.Skip(r) {
				next.ServeHTTP(w, r)
				return
			}

			r = r.WithContext(traceorb.ContextWithStore(r.Context()))
			body := traceorb.ReadAndRestoreBody(r, client.MaxBodyBytes())
			rec := newResponseRecorder(w, client.MaxBodyBytes())
			defer client.ObserveHTTP(r, rec.status, rec.body, body, opts)
			next.ServeHTTP(rec, r)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	raw    []byte
	body   any
	max    int
}

func newResponseRecorder(w http.ResponseWriter, max int) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, max: max}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	if len(r.raw) < r.max {
		take := r.max - len(r.raw)
		if take > len(b) {
			take = len(b)
		}

		r.raw = append(r.raw, b[:take]...)
		r.body = traceorb.DecodeCapturedBody(r.raw)
	}

	return r.ResponseWriter.Write(b)
}
