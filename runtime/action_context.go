package runtime

import (
	"context"
	"time"
)

// NoopActionCache is a do-nothing cache satisfying def.ActionCache.
// Use in tests or environments where no cache is configured.
type NoopActionCache struct{}

func (NoopActionCache) Get(_ context.Context, _ string, _ any) error            { return nil }
func (NoopActionCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error { return nil }
func (NoopActionCache) Delete(_ context.Context, _ string) error                { return nil }
func (NoopActionCache) DeletePrefix(_ context.Context, _ string) error          { return nil }
