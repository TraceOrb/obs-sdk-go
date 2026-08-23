package internal

const (
	Redacted              = "[redacted]"
	MaxBodyBytes          = 32 * 1024
	MaxIngestBatch        = 100
	MaxEventsPerRequest   = 50
	MaxTagsPerRequest     = 16
	DefaultMaxQueue       = 1000
	DefaultFlushInterval  = 1000
	DefaultFetchTimeoutMs = 10_000
	MaxExtraRedactKeys    = 32
	maxErrorMessageChars  = 512
	maxStackChars         = 2048
	maxRedactDepth        = 8

	maxBodyBytes          = MaxBodyBytes
	maxIngestBatch        = MaxIngestBatch
	maxEventsPerRequest   = MaxEventsPerRequest
	maxTagsPerRequest     = MaxTagsPerRequest
	defaultMaxQueue       = DefaultMaxQueue
	defaultFlushInterval  = DefaultFlushInterval
	defaultFetchTimeoutMs = DefaultFetchTimeoutMs
	maxExtraRedactKeys    = MaxExtraRedactKeys
)

var redactSensitiveKeys = map[string]struct{}{
	"authorization": {},
	"cookie":        {},
	"set-cookie":    {},
	"password":      {},
	"token":         {},
	"secret":        {},
	"api_key":       {},
	"apikey":        {},
}

var DefaultRetryDelaysMs = []int{200, 800}

var defaultRetryDelaysMs = DefaultRetryDelaysMs
