package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/broker"
)

func TestReplaySamplesPublishesRawAndCEXNormalizedRows(t *testing.T) {
	publisher := &broker.MemoryPublisher{}
	result, err := ReplaySamples(
		context.Background(),
		filepath.Join("..", "..", "..", "predictor", "samples"),
		publisher,
	)
	if err != nil {
		t.Fatalf("ReplaySamples returned error: %v", err)
	}
	if result.PublishedRaw != 21 || len(publisher.Raw) != 21 {
		t.Fatalf("published raw = %d memory=%d, want 21", result.PublishedRaw, len(publisher.Raw))
	}
	if result.PublishedNormalized != 20 || len(publisher.Normalized) != 20 {
		t.Fatalf("published normalized = %d memory=%d, want 20", result.PublishedNormalized, len(publisher.Normalized))
	}
	for _, row := range publisher.Normalized {
		if row.IsDEX() {
			t.Fatalf("DEX row was published: %#v", row)
		}
	}
}
