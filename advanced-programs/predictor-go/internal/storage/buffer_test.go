package storage

import (
	"context"
	"testing"
	"time"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

func TestBufferedWriterFlushesByBatchSize(t *testing.T) {
	sink := &MemorySink{}
	writer := NewBufferedWriter(sink, 2, time.Hour)
	if err := writer.AddRaw(context.Background(), contract.RawPayload{Exchange: "Binance"}); err != nil {
		t.Fatal(err)
	}
	if len(sink.Raw) != 0 {
		t.Fatalf("raw rows flushed too early")
	}
	if err := writer.AddNormalized(context.Background(), contract.NormalizedOrderBook{Exchange: "Binance"}); err != nil {
		t.Fatal(err)
	}
	if len(sink.Raw) != 1 || len(sink.Normalized) != 1 {
		t.Fatalf("sink = %#v", sink)
	}
}
