package middleware

import (
	"net/http"

	traceorb "github.com/TraceOrb/obs-sdk-go"
)

func Recovery(client *traceorb.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				traceorb.RecordError(r, recovered)
				panic(recovered)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func RecordError(r *http.Request, err any) {
	traceorb.RecordError(r, err)
}
