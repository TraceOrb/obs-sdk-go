package internal

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
)

type CaptureMeta struct {
	Service      string
	Env          string
	MaxBodyBytes int
	RedactKeys   []string
}

type ObserveOptions struct {
	ResolveTags         func(r *http.Request) map[string]string
	ResolveUserID       func(r *http.Request) string
	ResolveRoutePattern func(r *http.Request) string
	RedactKeys          []string
	ResolveRedactKeys   func(r *http.Request) []string
}

func IngestFromRequest(
	r *http.Request,
	status int,
	responseBody any,
	requestBody any,
	store *Store,
	meta CaptureMeta,
	opts ObserveOptions,
) IngestRequest {
	extraTags := map[string]string{}
	if opts.ResolveTags != nil {
		if tags := opts.ResolveTags(r); tags != nil {
			extraTags = tags
		}
	}

	userID := ""
	if opts.ResolveUserID != nil {
		userID = opts.ResolveUserID(r)
	}

	var resolvedRedact []string
	if opts.ResolveRedactKeys != nil {
		resolvedRedact = opts.ResolveRedactKeys(r)
	}

	path := r.URL.RequestURI()
	if path == "" {
		path = "/"
	}

	routePattern := path
	if opts.ResolveRoutePattern != nil {
		if resolved := opts.ResolveRoutePattern(r); resolved != "" {
			routePattern = resolved
		}
	}

	if responseBody != nil {
		SetResponseBody(store, responseBody)
	}

	return IngestFromCapture(CapturedHTTP{
		method:          r.Method,
		path:            path,
		routePattern:    routePattern,
		statusCode:      status,
		query:           queryToMap(r.URL.Query()),
		headers:         headersToMap(r.Header),
		body:            requestBody,
		ip:              requestIP(r),
		userAgent:       r.UserAgent(),
		extraTags:       extraTags,
		extraRedactKeys: MergeRedactKeys([][]string{opts.RedactKeys, resolvedRedact}),
		userID:          userID,
	}, store, meta)
}

func ReadAndRestoreBody(r *http.Request, max int) any {
	if r.Body == nil {
		return nil
	}

	limited := io.LimitReader(r.Body, int64(max)+1)
	raw, err := io.ReadAll(limited)
	_ = r.Body.Close()
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return nil
	}

	if len(raw) > max {
		raw = raw[:max]
	}

	r.Body = io.NopCloser(bytes.NewReader(raw))
	if len(raw) == 0 {
		return nil
	}

	return DecodeCapturedBody(raw)
}

func DecodeCapturedBody(raw []byte) any {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err == nil {
		return parsed
	}

	return string(raw)
}

func queryToMap(values url.Values) any {
	if len(values) == 0 {
		return nil
	}

	out := make(map[string]any, len(values))
	for key, list := range values {
		if len(list) == 1 {
			out[key] = list[0]
			continue
		}

		out[key] = list
	}

	return out
}

func headersToMap(header http.Header) map[string]any {
	out := make(map[string]any, len(header))
	for key, list := range header {
		if len(list) == 1 {
			out[key] = list[0]
			continue
		}

		out[key] = list
	}

	return out
}

func requestIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}
