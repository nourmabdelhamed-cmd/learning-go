//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/broker"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/storage"
)

func TestRedisAndClickHouseRoundTrip(t *testing.T) {
	if os.Getenv("PREDICTOR_INTEGRATION") != "1" {
		t.Skip("set PREDICTOR_INTEGRATION=1 after starting docker compose")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw := contract.RawPayload{
		SchemaVersion:    contract.SchemaVersion,
		Exchange:         "integration",
		InstrumentType:   "spot",
		ProductType:      "clob",
		ConnectionID:     "integration-test",
		ReceivedTSMS:     contract.NowMillis(),
		PayloadFormat:    "json",
		PayloadTruncated: "false",
		StreamID:         "integration.raw",
		Payload:          `{"bids":[["1","2"]],"asks":[["3","4"]]}`,
	}
	normalized := contract.NormalizedOrderBook{
		SchemaVersion:  contract.SchemaVersion,
		ProductType:    "clob",
		InstrumentType: "spot",
		Exchange:       "integration",
		Symbol:         "BTCUSDT",
		Base:           "BTC",
		Quote:          "USDT",
		InstrumentID:   "BTCUSDT",
		ReceivedTSMS:   contract.NowMillis(),
		StreamID:       "integration.normalized",
		BidsJSON:       `[["1","2"]]`,
		AsksJSON:       `[["3","4"]]`,
	}

	options, err := redis.ParseURL(redisURL())
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}
	redisClient := redis.NewClient(options)
	defer redisClient.Close()

	redisBroker := broker.NewRedisPublisher(redisClient, contract.RawRedisStream, contract.NormalizedRedisStream, 1000)

	if _, err := redisBroker.PublishRaw(ctx, raw); err != nil {
		t.Fatalf("publish raw: %v", err)
	}
	if _, err := redisBroker.PublishNormalized(ctx, normalized); err != nil {
		t.Fatalf("publish normalized: %v", err)
	}

	ch, err := storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
		Addr:      clickHouseAddr(),
		Database:  getenv("CLICKHOUSE_DATABASE", "orderbooks"),
		Username:  getenv("CLICKHOUSE_USER", "default"),
		Password:  os.Getenv("CLICKHOUSE_PASSWORD"),
		RawTable:  contract.RawClickHouseTable,
		NormTable: contract.NormalizedClickHouseTable,
	})
	if err != nil {
		t.Fatalf("open clickhouse: %v", err)
	}
	defer ch.Close()

	if err := ch.WriteRaw(ctx, []contract.RawPayload{raw}); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	if err := ch.WriteNormalized(ctx, []contract.NormalizedOrderBook{normalized}); err != nil {
		t.Fatalf("write normalized: %v", err)
	}
}

func redisURL() string {
	return getenv("REDIS_URL", "redis://localhost:6379/0")
}

func clickHouseAddr() []string {
	value := getenv("CLICKHOUSE_ADDR", "localhost:9000")
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
