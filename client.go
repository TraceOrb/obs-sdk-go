package traceorb

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/TraceOrb/obs-sdk-go/internal"
)

type Options struct {
	IngestURL       string
	WriteKey        string
	Service         string
	Env             string
	MaxBodyBytes    int
	MaxQueue        int
	FlushIntervalMs int
	RetryDelaysMs   []int
	FetchTimeoutMs  int
	HTTP            HTTPDoer
	OnDrop          OnDropFunc
	RedactKeys      []string
	SampleRate      *float64
	Capture         Capture
	Routes          map[string]RoutePolicy
}

type Client struct {
	service      string
	env          string
	maxBodyBytes int
	redactKeys   []string
	policy       ClientPolicy
	batch        *internal.Batch
}

func New(opts Options) (*Client, error) {
	if opts.IngestURL == "" {
		return nil, errors.New("ingest URL is required")
	}

	if opts.WriteKey == "" {
		return nil, errors.New("write key is required")
	}

	if opts.Service == "" {
		return nil, errors.New("service is required")
	}

	if opts.Env == "" {
		return nil, errors.New("env is required")
	}

	policy, err := ParseClientPolicy(opts)
	if err != nil {
		return nil, err
	}

	maxQueue := opts.MaxQueue
	if maxQueue == 0 {
		maxQueue = internal.DefaultMaxQueue
	}

	maxBytes := opts.MaxBodyBytes
	if maxBytes == 0 {
		maxBytes = internal.MaxBodyBytes
	}

	flushInterval := time.Duration(opts.FlushIntervalMs) * time.Millisecond
	if opts.FlushIntervalMs == 0 {
		if opts.HTTP == nil {
			flushInterval = time.Duration(internal.DefaultFlushInterval) * time.Millisecond
		}
	}

	retryMs := opts.RetryDelaysMs
	if retryMs == nil {
		retryMs = internal.DefaultRetryDelaysMs
	}

	fetchTimeout := time.Duration(opts.FetchTimeoutMs) * time.Millisecond
	if opts.FetchTimeoutMs == 0 {
		fetchTimeout = time.Duration(internal.DefaultFetchTimeoutMs) * time.Millisecond
	}

	onDrop := opts.OnDrop
	if onDrop == nil {
		onDrop = internal.DefaultOnDrop
	}

	httpDoer := opts.HTTP
	if httpDoer == nil {
		httpDoer = &http.Client{Timeout: fetchTimeout}
	}

	batch := internal.NewBatch(internal.BatchOptions{
		IngestURL:     opts.IngestURL,
		WriteKey:      opts.WriteKey,
		MaxQueue:      maxQueue,
		FlushSize:     internal.DefaultFlushSize(maxQueue),
		FlushInterval: flushInterval,
		RetryDelays:   internal.DelaysFromMs(retryMs),
		FetchTimeout:  fetchTimeout,
		HTTP:          httpDoer,
		OnDrop:        onDrop,
	})

	return &Client{
		service:      opts.Service,
		env:          opts.Env,
		maxBodyBytes: maxBytes,
		redactKeys:   internal.MergeRedactKeys([][]string{opts.RedactKeys}),
		policy:       policy,
		batch:        batch,
	}, nil
}

func (c *Client) Service() string {
	return c.service
}

func (c *Client) Env() string {
	return c.env
}

func (c *Client) MaxBodyBytes() int {
	return c.maxBodyBytes
}

func (c *Client) RedactKeys() []string {
	return c.redactKeys
}

func (c *Client) Step(ctx context.Context, name string, attrs map[string]string, options *StepOptions) {
	internal.Step(internal.StoreFrom(ctx), name, attrs, options)
}

func (c *Client) RequestID(ctx context.Context) string {
	return internal.StoreFrom(ctx).RequestID()
}

func (c *Client) SetTags(ctx context.Context, tags map[string]string) {
	internal.SetTags(internal.StoreFrom(ctx), tags)
}

func (c *Client) Redact(ctx context.Context, keys []string) {
	internal.Redact(internal.StoreFrom(ctx), keys)
}

func (c *Client) SetErrorMessage(ctx context.Context, message string) {
	internal.SetErrorMessage(internal.StoreFrom(ctx), message)
}

func (c *Client) Enqueue(request IngestRequest) {
	c.batch.Enqueue(request)
}

func (c *Client) Flush() {
	c.batch.Flush()
}

func (c *Client) Close() {
	c.batch.Close()
}

func (c *Client) captureMeta(capture Capture) internal.CaptureMeta {
	return internal.CaptureMeta{
		Service:      c.service,
		Env:          c.env,
		MaxBodyBytes: c.maxBodyBytes,
		RedactKeys:   c.redactKeys,
		Capture: internal.FieldCapture{
			Headers:      string(capture.Headers),
			Query:        string(capture.Query),
			RequestBody:  string(capture.RequestBody),
			ResponseBody: string(capture.ResponseBody),
		},
	}
}

func (c *Client) policyForRoute(routePattern string) ResolvedRoutePolicy {
	return PolicyForRoute(c.policy, routePattern)
}
