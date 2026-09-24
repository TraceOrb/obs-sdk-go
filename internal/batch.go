package internal

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type DropReason string

const (
	DropInvalidResponse DropReason = "invalid_response"
	DropRetryExhausted  DropReason = "retry_exhausted"
	DropNetwork         DropReason = "network"
)

type sendAction string

const (
	sendOK    sendAction = "ok"
	sendDrop  sendAction = "drop"
	sendRetry sendAction = "retry"
)

var actionByStatus = map[int]sendAction{
	200: sendOK,
	202: sendOK,
	400: sendDrop,
	401: sendDrop,
	402: sendDrop,
	413: sendDrop,
	429: sendRetry,
	503: sendRetry,
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type OnDropFunc func(reason DropReason)

type Batch struct {
	ingestURL     string
	writeKey      string
	maxQueue      int
	flushSize     int
	flushInterval time.Duration
	retryDelays   []time.Duration
	fetchTimeout  time.Duration
	http          HTTPDoer
	onDrop        OnDropFunc
	mu            sync.Mutex
	flushCond     *sync.Cond
	queue         []IngestRequest
	inFlight      int
	timerStop     chan struct{}
	timerOnce     sync.Once
	timerStarted  bool
}

func DefaultFlushSize(maxQueue int) int {
	if maxQueue < maxIngestBatch {
		return maxQueue
	}

	return maxIngestBatch
}

func DefaultOnDrop(reason DropReason) {
	log.Println("[obs]", string(reason))
}

func actionForStatus(status int) sendAction {
	mapped, ok := actionByStatus[status]
	if ok {
		return mapped
	}

	if status >= 500 {
		return sendRetry
	}

	return sendDrop
}

func NewBatch(opts BatchOptions) *Batch {
	batch := &Batch{
		ingestURL:     opts.IngestURL,
		writeKey:      opts.WriteKey,
		maxQueue:      opts.MaxQueue,
		flushSize:     opts.FlushSize,
		flushInterval: opts.FlushInterval,
		retryDelays:   opts.RetryDelays,
		fetchTimeout:  opts.FetchTimeout,
		http:          opts.HTTP,
		onDrop:        opts.OnDrop,
		queue:         make([]IngestRequest, 0, opts.MaxQueue),
		timerStop:     make(chan struct{}),
	}
	batch.flushCond = sync.NewCond(&batch.mu)

	return batch
}

type BatchOptions struct {
	IngestURL     string
	WriteKey      string
	MaxQueue      int
	FlushSize     int
	FlushInterval time.Duration
	RetryDelays   []time.Duration
	FetchTimeout  time.Duration
	HTTP          HTTPDoer
	OnDrop        OnDropFunc
}

func (b *Batch) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.queue)
}

func (b *Batch) Enqueue(request IngestRequest) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.queue) >= b.maxQueue {
		b.queue = b.queue[1:]
	}
	b.queue = append(b.queue, request)
	b.ensureTimerLocked()

	if len(b.queue) < b.flushSize {
		return
	}

	b.launchAvailableLocked()
}

func (b *Batch) ensureTimerLocked() {
	if b.flushInterval == 0 {
		return
	}

	if b.timerStarted {
		return
	}

	b.timerStarted = true
	go b.loopTimer()
}

func (b *Batch) loopTimer() {
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.timerStop:
			return
		case <-ticker.C:
			b.Flush()
		}
	}
}

func (b *Batch) Flush() {
	b.mu.Lock()
	for {
		b.launchAvailableLocked()
		if b.inFlight == 0 {
			b.mu.Unlock()
			return
		}
		b.flushCond.Wait()
	}
}

func (b *Batch) launchAvailableLocked() {
	for b.inFlight < maxInFlight && len(b.queue) > 0 {
		b.launchLocked()
	}
}

func (b *Batch) launchLocked() {
	if b.inFlight >= maxInFlight {
		return
	}

	batch := b.takeBatchLocked()
	if len(batch) == 0 {
		return
	}

	b.inFlight++
	go func(requests []IngestRequest) {
		defer func() {
			b.mu.Lock()
			b.inFlight--
			b.flushCond.Broadcast()
			b.mu.Unlock()
		}()
		defer func() {
			_ = recover()
		}()
		b.sendWithRetry(requests)
	}(batch)
}

func (b *Batch) takeBatchLocked() []IngestRequest {
	size := b.flushSize
	if size > len(b.queue) {
		size = len(b.queue)
	}

	taken := make([]IngestRequest, size)
	copy(taken, b.queue[:size])
	b.queue = b.queue[size:]
	return taken
}

func (b *Batch) Close() {
	b.mu.Lock()
	started := b.timerStarted
	b.mu.Unlock()

	if started {
		b.timerOnce.Do(func() {
			close(b.timerStop)
		})
	}

	b.Flush()
}

func (b *Batch) sendWithRetry(requests []IngestRequest) {
	attempt := 0

	for {
		action := b.sendOnce(requests)
		if action == sendOK {
			return
		}

		if action == sendDrop {
			b.onDrop(DropInvalidResponse)
			return
		}

		if attempt >= len(b.retryDelays) {
			b.onDrop(DropRetryExhausted)
			return
		}

		delay := b.retryDelays[attempt]
		if delay > 0 {
			time.Sleep(delay)
		}
		attempt += 1
	}
}

func (b *Batch) sendOnce(requests []IngestRequest) sendAction {
	payload := IngestPayload{Requests: requests}
	body, err := json.Marshal(payload)
	if err != nil {
		return sendDrop
	}

	req, err := http.NewRequest(http.MethodPost, b.ingestURL, bytes.NewReader(body))
	if err != nil {
		return sendRetry
	}

	req.Header.Set("authorization", "Bearer "+b.writeKey)
	req.Header.Set("content-type", "application/json")

	client := b.http
	if client == nil {
		client = &http.Client{Timeout: b.fetchTimeout}
	}

	res, err := client.Do(req)
	if err != nil {
		return sendRetry
	}

	io.Copy(io.Discard, res.Body)
	res.Body.Close()

	return actionForStatus(res.StatusCode)
}

func DelaysFromMs(values []int) []time.Duration {
	out := make([]time.Duration, len(values))
	for i, ms := range values {
		out[i] = time.Duration(ms) * time.Millisecond
	}

	return out
}
