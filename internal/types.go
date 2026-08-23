package internal

type EventLevel string

const (
	EventLevelInfo  EventLevel = "info"
	EventLevelWarn  EventLevel = "warn"
	EventLevelError EventLevel = "error"
)

type IngestEvent struct {
	Seq        int               `json:"seq"`
	Timestamp  string            `json:"timestamp"`
	Name       string            `json:"name"`
	Level      EventLevel        `json:"level"`
	DurationMs *int              `json:"durationMs,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
}

type IngestRequest struct {
	RequestID          string            `json:"requestId"`
	Timestamp          string            `json:"timestamp"`
	Method             string            `json:"method"`
	Path               string            `json:"path"`
	RoutePattern       string            `json:"routePattern"`
	StatusCode         int               `json:"statusCode"`
	DurationMs         int               `json:"durationMs"`
	Service            string            `json:"service"`
	Env                string            `json:"env"`
	Tags               map[string]string `json:"tags,omitempty"`
	UserID             string            `json:"userId,omitempty"`
	IP                 string            `json:"ip,omitempty"`
	UserAgent          string            `json:"userAgent,omitempty"`
	ErrorMessage       string            `json:"errorMessage,omitempty"`
	QueryJSON          string            `json:"queryJson,omitempty"`
	RequestHeadersJSON string            `json:"requestHeadersJson,omitempty"`
	RequestBodyJSON    string            `json:"requestBodyJson,omitempty"`
	ResponseBodyJSON   string            `json:"responseBodyJson,omitempty"`
	Events             []IngestEvent     `json:"events,omitempty"`
}

type IngestPayload struct {
	Requests []IngestRequest `json:"requests"`
}

type StepOptions struct {
	Level      EventLevel
	DurationMs *int
}
