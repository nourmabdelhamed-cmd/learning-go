package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

type Publisher interface {
	PublishRaw(ctx context.Context, row contract.RawPayload) (string, error)
	PublishNormalized(ctx context.Context, row contract.NormalizedOrderBook) (string, error)
}

type RedisPublisher struct {
	client           *redis.Client
	rawStream        string
	normalizedStream string
	maxLen           int64
}

func NewRedisPublisher(client *redis.Client, rawStream, normalizedStream string, maxLen int64) *RedisPublisher {
	return &RedisPublisher{
		client:           client,
		rawStream:        defaultString(rawStream, contract.RawRedisStream),
		normalizedStream: defaultString(normalizedStream, contract.NormalizedRedisStream),
		maxLen:           maxLen,
	}
}

func (p *RedisPublisher) PublishRaw(ctx context.Context, row contract.RawPayload) (string, error) {
	args := &redis.XAddArgs{Stream: p.rawStream, Values: row.RedisFields()}
	if p.maxLen > 0 {
		args.MaxLen = p.maxLen
		args.Approx = true
	}
	return p.client.XAdd(ctx, args).Result()
}

func (p *RedisPublisher) PublishNormalized(ctx context.Context, row contract.NormalizedOrderBook) (string, error) {
	args := &redis.XAddArgs{Stream: p.normalizedStream, Values: row.RedisFields()}
	if p.maxLen > 0 {
		args.MaxLen = p.maxLen
		args.Approx = true
	}
	return p.client.XAdd(ctx, args).Result()
}

type MemoryPublisher struct {
	Raw        []contract.RawPayload
	Normalized []contract.NormalizedOrderBook
}

func (p *MemoryPublisher) PublishRaw(_ context.Context, row contract.RawPayload) (string, error) {
	id := fmt.Sprintf("memory-raw-%d", len(p.Raw)+1)
	p.Raw = append(p.Raw, row)
	return id, nil
}

func (p *MemoryPublisher) PublishNormalized(_ context.Context, row contract.NormalizedOrderBook) (string, error) {
	id := fmt.Sprintf("memory-normalized-%d", len(p.Normalized)+1)
	p.Normalized = append(p.Normalized, row)
	return id, nil
}

type MessageKind string

const (
	RawMessage        MessageKind = "raw"
	NormalizedMessage MessageKind = "normalized"
)

type StreamMessage struct {
	Kind   MessageKind
	Stream string
	ID     string
	Fields map[string]string
}

type Consumer interface {
	Read(ctx context.Context, count int64, block time.Duration) ([]StreamMessage, error)
	Ack(ctx context.Context, messages []StreamMessage) error
}

type RedisConsumer struct {
	client           *redis.Client
	rawStream        string
	normalizedStream string
	group            string
	consumer         string
}

func NewRedisConsumer(client *redis.Client, rawStream, normalizedStream, group, consumer string) *RedisConsumer {
	return &RedisConsumer{
		client:           client,
		rawStream:        defaultString(rawStream, contract.RawRedisStream),
		normalizedStream: defaultString(normalizedStream, contract.NormalizedRedisStream),
		group:            defaultString(group, "predictor-go-writers"),
		consumer:         defaultString(consumer, "writer"),
	}
}

func (c *RedisConsumer) EnsureGroups(ctx context.Context) error {
	for _, stream := range []string{c.rawStream, c.normalizedStream} {
		if err := c.client.XGroupCreateMkStream(ctx, stream, c.group, "0").Err(); err != nil {
			if !isBusyGroup(err) {
				return err
			}
		}
	}
	return nil
}

func (c *RedisConsumer) Read(ctx context.Context, count int64, block time.Duration) ([]StreamMessage, error) {
	response, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{c.rawStream, c.normalizedStream, ">", ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []StreamMessage
	for _, stream := range response {
		kind := RawMessage
		if stream.Stream == c.normalizedStream {
			kind = NormalizedMessage
		}
		for _, message := range stream.Messages {
			out = append(out, StreamMessage{
				Kind:   kind,
				Stream: stream.Stream,
				ID:     message.ID,
				Fields: stringMap(message.Values),
			})
		}
	}
	return out, nil
}

func (c *RedisConsumer) Ack(ctx context.Context, messages []StreamMessage) error {
	byStream := map[string][]string{}
	for _, message := range messages {
		byStream[message.Stream] = append(byStream[message.Stream], message.ID)
	}
	for stream, ids := range byStream {
		if len(ids) == 0 {
			continue
		}
		if err := c.client.XAck(ctx, stream, c.group, ids...).Err(); err != nil {
			return err
		}
	}
	return nil
}

func stringMap(values map[string]any) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = fmt.Sprint(value)
	}
	return out
}

func isBusyGroup(err error) bool {
	return err != nil && len(err.Error()) >= 9 && err.Error()[:9] == "BUSYGROUP"
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
