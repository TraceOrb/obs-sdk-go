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

func IngestFromCapture(http CapturedHTTP, store *Store, meta CaptureMeta) IngestRequest {
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
	assignOptional(&request.QueryJSON, toBodyJSON(http.query, maxBytes, extraKeys))
	assignOptional(&request.RequestHeadersJSON, toHeadersJSON(http.headers, maxBytes, extraKeys))
	assignOptional(&request.RequestBodyJSON, toBodyJSON(http.body, maxBytes, extraKeys))
	assignOptional(&request.ResponseBodyJSON, toBodyJSON(store.responseBody, maxBytes, extraKeys))

	if len(store.events) > 0 {
		request.Events = store.events
	}

	return request
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
