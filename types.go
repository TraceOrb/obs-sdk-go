package traceorb

import "github.com/TraceOrb/obs-sdk-go/internal"

type EventLevel = internal.EventLevel
type IngestEvent = internal.IngestEvent
type IngestRequest = internal.IngestRequest
type IngestPayload = internal.IngestPayload
type StepOptions = internal.StepOptions
type DropReason = internal.DropReason
type HTTPDoer = internal.HTTPDoer
type OnDropFunc = internal.OnDropFunc

const (
	EventLevelInfo  = internal.EventLevelInfo
	EventLevelWarn  = internal.EventLevelWarn
	EventLevelError = internal.EventLevelError
	Redacted        = internal.Redacted
	MaxBodyBytes    = internal.MaxBodyBytes

	DropInvalidResponse = internal.DropInvalidResponse
	DropRetryExhausted  = internal.DropRetryExhausted
	DropNetwork         = internal.DropNetwork
)
