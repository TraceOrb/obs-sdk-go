package gin

import (
	"net/http"

	traceorb "github.com/TraceOrb/obs-sdk-go"
	"github.com/gin-gonic/gin"
)

type Options struct {
	Skip              func(*gin.Context) bool
	ResolveTags       func(*gin.Context) map[string]string
	ResolveUserID     func(*gin.Context) string
	RedactKeys        []string
	ResolveRedactKeys func(*gin.Context) []string
}

func Middleware(client *traceorb.Client, opts Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		if opts.Skip != nil && opts.Skip(c) {
			c.Next()
			return
		}

		c.Request = c.Request.WithContext(traceorb.ContextWithStore(c.Request.Context()))
		requestBody := traceorb.ReadAndRestoreBody(c.Request, client.MaxBodyBytes())
		writer := newBodyWriter(c.Writer, client.MaxBodyBytes())
		c.Writer = writer
		c.Next()

		routePattern := c.FullPath()
		if routePattern == "" {
			routePattern = c.Request.URL.RequestURI()
		}

		traceorbOpts := traceorb.MiddlewareOptions{
			RedactKeys: opts.RedactKeys,
			ResolveRoutePattern: func(_ *http.Request) string {
				return routePattern
			},
		}

		if opts.ResolveTags != nil {
			traceorbOpts.ResolveTags = func(_ *http.Request) map[string]string {
				return opts.ResolveTags(c)
			}
		}

		if opts.ResolveUserID != nil {
			traceorbOpts.ResolveUserID = func(_ *http.Request) string {
				return opts.ResolveUserID(c)
			}
		}

		if opts.ResolveRedactKeys != nil {
			traceorbOpts.ResolveRedactKeys = func(_ *http.Request) []string {
				return opts.ResolveRedactKeys(c)
			}
		}

		client.ObserveHTTP(c.Request, c.Writer.Status(), writer.body, requestBody, traceorbOpts)
	}
}

func Recovery(client *traceorb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			traceorb.RecordError(c.Request, recovered)
			panic(recovered)
		}()

		c.Next()
	}
}

func ErrorHandler(client *traceorb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}

		traceorb.RecordError(c.Request, c.Errors.Last().Err)
	}
}

type bodyWriter struct {
	gin.ResponseWriter
	raw  []byte
	body any
	max  int
}

func newBodyWriter(w gin.ResponseWriter, max int) *bodyWriter {
	return &bodyWriter{ResponseWriter: w, max: max}
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	if len(w.raw) < w.max {
		take := w.max - len(w.raw)
		if take > len(b) {
			take = len(b)
		}

		w.raw = append(w.raw, b[:take]...)
		w.body = traceorb.DecodeCapturedBody(w.raw)
	}

	return w.ResponseWriter.Write(b)
}
