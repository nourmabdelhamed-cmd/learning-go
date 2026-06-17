package storage

import (
	"context"
	"time"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

type BufferedWriter struct {
	Sink          Sink
	BatchSize     int
	FlushInterval time.Duration
	raw           []contract.RawPayload
	normalized    []contract.NormalizedOrderBook
	lastFlush     time.Time
}

func NewBufferedWriter(sink Sink, batchSize int, flushInterval time.Duration) *BufferedWriter {
	if batchSize <= 0 {
		batchSize = 1000
	}
	if flushInterval <= 0 {
		flushInterval = time.Second
	}
	return &BufferedWriter{
		Sink:          sink,
		BatchSize:     batchSize,
		FlushInterval: flushInterval,
		lastFlush:     time.Now(),
	}
}

func (w *BufferedWriter) AddRaw(ctx context.Context, row contract.RawPayload) error {
	w.raw = append(w.raw, row)
	return w.flushIfNeeded(ctx)
}

func (w *BufferedWriter) AddNormalized(ctx context.Context, row contract.NormalizedOrderBook) error {
	w.normalized = append(w.normalized, row)
	return w.flushIfNeeded(ctx)
}

func (w *BufferedWriter) Flush(ctx context.Context) error {
	raw := w.raw
	normalized := w.normalized
	w.raw = nil
	w.normalized = nil
	if len(raw) > 0 {
		if err := w.Sink.WriteRaw(ctx, raw); err != nil {
			w.raw = append(raw, w.raw...)
			return err
		}
	}
	if len(normalized) > 0 {
		if err := w.Sink.WriteNormalized(ctx, normalized); err != nil {
			w.normalized = append(normalized, w.normalized...)
			return err
		}
	}
	w.lastFlush = time.Now()
	return nil
}

func (w *BufferedWriter) flushIfNeeded(ctx context.Context) error {
	if len(w.raw)+len(w.normalized) >= w.BatchSize {
		return w.Flush(ctx)
	}
	if time.Since(w.lastFlush) >= w.FlushInterval {
		return w.Flush(ctx)
	}
	return nil
}
