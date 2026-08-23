package traceorb

import (
	"context"
	"testing"
)

func TestStepIsNoopWithoutStore(t *testing.T) {
	t.Parallel()

	client := testClient(t, nil)
	defer client.Close()

	client.Step(context.Background(), "handler", nil, nil)
	if client.RequestID(context.Background()) != "" {
		t.Fatal("expected empty request id")
	}
}

func TestSetTagsRedactAreNoopWithoutStore(t *testing.T) {
	t.Parallel()

	client := testClient(t, nil)
	defer client.Close()
	client.SetTags(context.Background(), map[string]string{"city": "6"})
	client.Redact(context.Background(), []string{"email"})
	client.SetErrorMessage(context.Background(), "timeout")
}

func TestStepRecordsEventsThroughClient(t *testing.T) {
	t.Parallel()

	client := testClient(t, nil)
	defer client.Close()
	ctx := ContextWithStore(context.Background())
	duration := 4
	client.Step(ctx, "db.query", map[string]string{"table": "users"}, &StepOptions{
		Level:      EventLevelInfo,
		DurationMs: &duration,
	})

	if client.RequestID(ctx) == "" {
		t.Fatal("expected request id")
	}
}
