package events

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONLPublisherWritesEnvelope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	pub, err := NewJSONLPublisher(path)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := Payload(map[string]string{"sample_id": "sample-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(context.Background(), "nuscenes.sample.v1", "sample-1", Envelope{
		EventType:      "sample",
		DatasetVersion: "fixture",
		SchemaVersion:  "manifest.v1",
		SampleID:       "sample-1",
		Payload:        payload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := pub.Close(); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "nuscenes.sample.v1") || len(pub.Events()) != 1 {
		t.Fatalf("event was not written correctly: %s", string(text))
	}
}
