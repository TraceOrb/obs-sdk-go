package internal

import "time"

type CapturedHTTP struct {
	method          string
	path            string
	routePattern    string
	statusCode      int
	query           any
	headers         map[string]any
	body            any
	ip              string
	userAgent       string
	extraTags       map[string]string
	extraRedactKeys []string
	userID          string
}

// StoreSnapshot is a detached copy of Store fields used for ingest.
// ObserveHTTP snapshots before the goroutine so Gin ErrorHandler (or other
// post-Next writers) cannot race the async stringify/enqueue path.
type StoreSnapshot struct {
	requestID    string
	startedAt    time.Time
	events       []IngestEvent
	tags         map[string]string
	redactKeys   []string
	errorMessage string
	responseBody any
}

func SnapshotStore(store *Store) StoreSnapshot {
	if store == nil {
		return StoreSnapshot{}
	}

	events := make([]IngestEvent, len(store.events))
	copy(events, store.events)

	return StoreSnapshot{
		requestID:    store.requestID,
		startedAt:    store.startedAt,
		events:       events,
		tags:         copyStringMap(store.tags),
		redactKeys:   append([]string{}, store.redactKeys...),
		errorMessage: store.errorMessage,
		responseBody: store.responseBody,
	}
}

func PrepareIngestStore(store *Store, responseBody any) StoreSnapshot {
	snap := SnapshotStore(store)
	if responseBody != nil {
		snap.responseBody = responseBody
	}
	return snap
}

func IngestFromCapture(http CapturedHTTP, store StoreSnapshot, meta CaptureMeta) IngestRequest {
	finishedAt := time.Now().UTC()
	durationMs := int(finishedAt.Sub(store.startedAt) / time.Millisecond)
	if durationMs < 0 {
		durationMs = 0
	}

	extraKeys := MergeRedactKeys([][]string{
		meta.RedactKeys,
		http.extraRedactKeys,
		store.redactKeys,
	})
	maxBytes := meta.MaxBodyBytes
	capture := resolveFieldCapture(meta.Capture)

	request := IngestRequest{
		RequestID:    store.requestID,
		Timestamp:    store.startedAt.Format(time.RFC3339Nano),
		Method:       http.method,
		Path:         http.path,
		RoutePattern: http.routePattern,
		StatusCode:   http.statusCode,
		DurationMs:   durationMs,
		Service:      meta.Service,
		Env:          meta.Env,
	}

	tags := mergeTags(store.tags, http.extraTags)
	if len(tags) > 0 {
		request.Tags = tags
	}

	assignOptional(&request.UserID, http.userID)
	assignOptional(&request.IP, http.ip)
	assignOptional(&request.UserAgent, http.userAgent)
	assignOptional(&request.ErrorMessage, store.errorMessage)

	if ShouldIncludeField(capture.Query, http.statusCode) {
		assignOptional(&request.QueryJSON, toBodyJSON(http.query, maxBytes, extraKeys))
	}
	if ShouldIncludeField(capture.Headers, http.statusCode) {
		assignOptional(&request.RequestHeadersJSON, toHeadersJSON(http.headers, maxBytes, extraKeys))
	}
	if ShouldIncludeField(capture.RequestBody, http.statusCode) {
		assignOptional(&request.RequestBodyJSON, toBodyJSON(http.body, maxBytes, extraKeys))
	}
	if ShouldIncludeField(capture.ResponseBody, http.statusCode) {
		assignOptional(&request.ResponseBodyJSON, toBodyJSON(store.responseBody, maxBytes, extraKeys))
	}

	if len(store.events) > 0 {
		request.Events = store.events
	}

	return request
}

func resolveFieldCapture(capture FieldCapture) FieldCapture {
	return FieldCapture{
		Headers:      defaultCaptureMode(capture.Headers),
		Query:        defaultCaptureMode(capture.Query),
		RequestBody:  defaultCaptureMode(capture.RequestBody),
		ResponseBody: defaultCaptureMode(capture.ResponseBody),
	}
}

func defaultCaptureMode(mode string) string {
	if mode == "" {
		return "always"
	}
	return mode
}

func ShouldIncludeField(mode string, statusCode int) bool {
	switch mode {
	case "never":
		return false
	case "errors":
		return statusCode >= 400
	default:
		return true
	}
}

func assignOptional(target *string, value string) {
	if value == "" {
		return
	}

	*target = value
}

func mergeTags(storeTags map[string]string, extra map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range storeTags {
		merged[key] = value
	}

	for key, value := range extra {
		merged[key] = value
	}

	if len(merged) == 0 {
		return nil
	}

	capped := map[string]string{}
	count := 0
	for key, value := range merged {
		if count >= maxTagsPerRequest {
			break
		}

		capped[key] = value
		count++
	}

	return capped
}
