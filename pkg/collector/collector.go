package collector

import (
	"context"

	"github.com/kanozad/dtop/pkg/types"
)

// Data represents collected monitor data.
type Data = types.Data

// Collector defines the data collection lifecycle.
type Collector interface {
	Init(ctx context.Context, cfg map[string]any) error
	Collect(ctx context.Context) (Data, error)
	Shutdown(ctx context.Context) error
}
