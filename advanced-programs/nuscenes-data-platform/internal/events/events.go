package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Envelope struct {
	EventID        string          `json:"event_id" parquet:"event_id"`
	EventType      string          `json:"event_type" parquet:"event_type"`
	Topic          string          `json:"topic" parquet:"topic"`
	Key            string          `json:"key" parquet:"key"`
	DatasetVersion string          `json:"dataset_version" parquet:"dataset_version"`
	SchemaVersion  string          `json:"schema_version" parquet:"schema_version"`
	SceneID        string          `json:"scene_id" parquet:"scene_id"`
	SampleID       string          `json:"sample_id" parquet:"sample_id"`
	SensorChannel  string          `json:"sensor_channel" parquet:"sensor_channel"`
	SourcePath     string          `json:"source_path" parquet:"source_path"`
	EventTimeUS    int64           `json:"event_time_us" parquet:"event_time_us"`
	Payload        json.RawMessage `json:"payload" parquet:"payload"`
}

type Publisher interface {
	Publish(ctx context.Context, topic string, key string, event Envelope) error
	Close() error
	Events() []Envelope
}

type JSONLPublisher struct {
	mu      sync.Mutex
	path    string
	file    *os.File
	writer  *bufio.Writer
	counter int
	events  []Envelope
}

func NewJSONLPublisher(path string) (*JSONLPublisher, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &JSONLPublisher{
		path:   path,
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

func (p *JSONLPublisher) Publish(_ context.Context, topic string, key string, event Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counter++
	event.Topic = topic
	event.Key = key
	if event.EventID == "" {
		event.EventID = fmt.Sprintf("%06d:%s:%s", p.counter, topic, key)
	}
	if event.EventTimeUS == 0 {
		event.EventTimeUS = time.Now().UnixMicro()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := p.writer.Write(append(payload, '\n')); err != nil {
		return err
	}
	p.events = append(p.events, event)
	return nil
}

func (p *JSONLPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.writer != nil {
		if err := p.writer.Flush(); err != nil {
			_ = p.file.Close()
			return err
		}
	}
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}

func (p *JSONLPublisher) Events() []Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]Envelope(nil), p.events...)
}

func Payload(value any) (json.RawMessage, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
