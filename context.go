package traceorb

import (
	"context"

	"github.com/TraceOrb/obs-sdk-go/internal"
)

func ContextWithStore(ctx context.Context) context.Context {
	return internal.WithStore(ctx, internal.NewStore())
}
