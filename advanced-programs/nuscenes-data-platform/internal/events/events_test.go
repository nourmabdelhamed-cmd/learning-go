package events

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestJSONLPublisherSupportsConcurrentPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	pub, err := NewJSONLPublisher(path)
	if err != nil {
		t.Fatal(err)
	}

	const count = 100
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload, err := Payload(map[string]int{"index": i})
			if err != nil {
				errs <- err
				return
			}
			errs <- pub.Publish(context.Background(), "nuscenes.sample.v1", fmt.Sprintf("sample-%d", i), Envelope{
				EventType:      "sample",
				DatasetVersion: "fixture",
				SchemaVersion:  "manifest.v1",
				SampleID:       fmt.Sprintf("sample-%d", i),
				Payload:        payload,
			})
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := pub.Close(); err != nil {
		t.Fatal(err)
	}

	if len(pub.Events()) != count {
		t.Fatalf("events = %d, want %d", len(pub.Events()), count)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if len(lines) != count {
		t.Fatalf("event log lines = %d, want %d", len(lines), count)
	}
}
