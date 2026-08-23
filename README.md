# traceorb-go

Go SDK for [Traceorb](https://traceorb.com). Send request traces from your API to your Traceorb workspace.

## Install

```bash
go get github.com/TraceOrb/obs-sdk-go
```

Gin:

```bash
go get github.com/TraceOrb/obs-sdk-go/gin
```

## Configure

Create a write key after you have a workspace. Sign up at [traceorb.com](https://traceorb.com). Set these in your app:

```
OBS_INGEST_URL=https://api.traceorb.com/v1/ingest
OBS_WRITE_KEY=
```

The SDK does not read the environment by itself. Pass the values into `traceorb.New`.

## Usage

```go
package main

import (
	"net/http"
	"os"

	"github.com/TraceOrb/obs-sdk-go"
	"github.com/TraceOrb/obs-sdk-go/middleware"
)

func main() {
	obs, err := traceorb.New(traceorb.Options{
		IngestURL: os.Getenv("OBS_INGEST_URL"),
		WriteKey:  os.Getenv("OBS_WRITE_KEY"),
		Service:   "orders-api",
		Env:       "production",
	})
	if err != nil {
		panic(err)
	}
	defer obs.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		obs.Step(r.Context(), "handler", nil, nil)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	handler := middleware.Middleware(obs, middleware.Options{
		ResolveTags: func(r *http.Request) map[string]string {
			return map[string]string{"tenant": r.Header.Get("X-Tenant")}
		},
	})(mux)

	http.ListenAndServe(":8080", handler)
}
```

Works with the standard library, Chi, and anything else that takes `http.Handler`.

If Traceorb is unreachable, your HTTP request still completes. Headers and body fields named `authorization`, `cookie`, `set-cookie`, `password`, `token`, `secret`, `api_key`, and `apikey` are replaced with `[redacted]`.

`routePattern` defaults to the request URI. Pass `ResolveRoutePattern` so Traceorb groups by template (`/orders/{id}`) instead of `/orders/123`.

Gin:

```go
package main

import (
	"os"

	"github.com/gin-gonic/gin"
	traceorb "github.com/TraceOrb/obs-sdk-go"
	traceorbgin "github.com/TraceOrb/obs-sdk-go/gin"
)

func main() {
	obs, err := traceorb.New(traceorb.Options{
		IngestURL: os.Getenv("OBS_INGEST_URL"),
		WriteKey:  os.Getenv("OBS_WRITE_KEY"),
		Service:   "orders-api",
		Env:       "production",
	})
	if err != nil {
		panic(err)
	}
	defer obs.Close()

	r := gin.New()
	r.Use(traceorbgin.Middleware(obs, traceorbgin.Options{}))
	r.Use(traceorbgin.ErrorHandler(obs))
	r.GET("/v1/orders/:id", func(c *gin.Context) {
		obs.Step(c.Request.Context(), "handler", nil, nil)
		c.JSON(200, gin.H{"ok": true})
	})
	r.Run(":8080")
}
```

Gin uses `FullPath()` as `routePattern` (`/v1/orders/:id`).

## Capture the request error (optional)

The middleware does not record panics or handler errors by itself. Mount `Recovery` around your handler (inside `Middleware`) and/or call `RecordError` when you handle an error yourself. The 500 then gets `errorMessage` and an `unhandled.error` step. Skip this and 4xx/5xx still ingest.

```go
handler := middleware.Middleware(obs, middleware.Options{})(
	middleware.Recovery(obs)(mux),
)
```

`Recovery` records the panic and re-panics. It does not change status or body. Put your own recovery outside if you want a 500 response.

When you catch an error in the handler:

```go
traceorb.RecordError(r, err)
```

Inside a request:

```go
obs.Step(r.Context(), "db.query", map[string]string{"table": "orders"}, nil)
obs.SetTags(r.Context(), map[string]string{"city": "4"})
obs.Redact(r.Context(), []string{"ssn"})
```

## Extra redaction

The built-in names always apply. Add more field names in any of these places. Names are case-insensitive. They apply to headers, query, and bodies.

On the client, for every request:

```go
obs, err := traceorb.New(traceorb.Options{
	IngestURL:  ingestURL,
	WriteKey:   writeKey,
	Service:    "orders-api",
	Env:        "production",
	RedactKeys: []string{"email", "cpf"},
})
```

On the middleware:

```go
middleware.Middleware(obs, middleware.Options{
	RedactKeys: []string{"email", "cpf"},
	ResolveRedactKeys: func(r *http.Request) []string {
		return []string{r.Header.Get("X-Redact")}
	},
})
```

Gin takes the same `RedactKeys` and `ResolveRedactKeys` options.

Inside a handler, for that request only:

```go
obs.Redact(r.Context(), []string{"ssn"})
```
