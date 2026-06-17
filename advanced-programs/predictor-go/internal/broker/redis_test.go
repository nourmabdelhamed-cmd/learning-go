package broker

import (
	"context"
	"testing"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

func TestMemoryPublisherRecordsRows(t *testing.T) {
	publisher := &MemoryPublisher{}
	if _, err := publisher.PublishRaw(context.Background(), contract.RawPayload{Exchange: "Binance"}); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.PublishNormalized(context.Background(), contract.NormalizedOrderBook{Exchange: "Binance"}); err != nil {
		t.Fatal(err)
	}
	if len(publisher.Raw) != 1 || len(publisher.Normalized) != 1 {
		t.Fatalf("publisher = %#v", publisher)
	}
}
