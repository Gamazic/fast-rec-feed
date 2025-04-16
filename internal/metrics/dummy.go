package metrics

import (
	"context"
	"sync/atomic"
)

type DummyMetrics struct {
	errsCount atomic.Int64
}

func NewDummyMetrics() *DummyMetrics {
	return &DummyMetrics{}
}

func (d *DummyMetrics) RecordFeedError(ctx context.Context, userId uint32, err error) {
	d.errsCount.Add(1)
}
