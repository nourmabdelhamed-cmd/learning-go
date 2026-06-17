package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/app"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/broker"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/metrics"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/storage"
)

func main() {
	redisURL := flag.String("redis-url", env("REDIS_URL", "redis://localhost:6379/0"), "Redis URL")
	rawStream := flag.String("raw-stream", env("RAW_REDIS_STREAM", contract.RawRedisStream), "raw Redis stream")
	normalizedStream := flag.String("normalized-stream", env("NORMALIZED_REDIS_STREAM", contract.NormalizedRedisStream), "normalized Redis stream")
	group := flag.String("group", env("REDIS_CONSUMER_GROUP", "predictor-go-writers"), "Redis consumer group")
	consumerName := flag.String("consumer", env("REDIS_CONSUMER_NAME", "writer"), "Redis consumer name")
	batchSize := flag.Int("batch-size", 1000, "writer batch size")
	flushInterval := flag.Duration("flush-interval", time.Second, "writer flush interval")
	metricsAddr := flag.String("metrics-addr", ":8080", "health and metrics address")
	sinkKind := flag.String("sink", env("PREDICTOR_SINK", "parquet"), "sink: parquet or clickhouse")
	parquetDir := flag.String("parquet-dir", env("PARQUET_OUTPUT_DIR", "output/orderbooks"), "Parquet output directory")
	clickhouseAddr := flag.String("clickhouse-addr", env("CLICKHOUSE_ADDR", "localhost:9000"), "ClickHouse native address")
	clickhouseDB := flag.String("clickhouse-database", env("CLICKHOUSE_DATABASE", "orderbooks"), "ClickHouse database")
	clickhouseUser := flag.String("clickhouse-user", env("CLICKHOUSE_USER", "default"), "ClickHouse user")
	clickhousePassword := flag.String("clickhouse-password", env("CLICKHOUSE_PASSWORD", ""), "ClickHouse password")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metricSet := metrics.New()
	go func() {
		if err := metrics.Serve(ctx, *metricsAddr, metricSet.Registry); err != nil {
			fmt.Fprintf(os.Stderr, "metrics server: %v\n", err)
		}
	}()

	exitIfError(run(ctx, writerOptions{
		RedisURL:           *redisURL,
		RawStream:          *rawStream,
		NormalizedStream:   *normalizedStream,
		Group:              *group,
		ConsumerName:       *consumerName,
		BatchSize:          *batchSize,
		FlushInterval:      *flushInterval,
		SinkKind:           *sinkKind,
		ParquetDir:         *parquetDir,
		ClickHouseAddr:     *clickhouseAddr,
		ClickHouseDatabase: *clickhouseDB,
		ClickHouseUser:     *clickhouseUser,
		ClickHousePassword: *clickhousePassword,
	}))
}

type writerOptions struct {
	RedisURL           string
	RawStream          string
	NormalizedStream   string
	Group              string
	ConsumerName       string
	BatchSize          int
	FlushInterval      time.Duration
	SinkKind           string
	ParquetDir         string
	ClickHouseAddr     string
	ClickHouseDatabase string
	ClickHouseUser     string
	ClickHousePassword string
}

func run(ctx context.Context, options writerOptions) error {
	redisOptions, err := redis.ParseURL(options.RedisURL)
	if err != nil {
		return err
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	consumer := broker.NewRedisConsumer(redisClient, options.RawStream, options.NormalizedStream, options.Group, options.ConsumerName)
	if err := consumer.EnsureGroups(ctx); err != nil {
		return err
	}

	sink, err := buildSink(ctx, options)
	if err != nil {
		return err
	}
	defer sink.Close()

	service := app.WriterService{
		Consumer: consumer,
		Writer:   storage.NewBufferedWriter(sink, options.BatchSize, options.FlushInterval),
	}
	return service.Run(ctx)
}

func buildSink(ctx context.Context, options writerOptions) (storage.Sink, error) {
	switch options.SinkKind {
	case "parquet":
		return storage.NewParquetSink(options.ParquetDir), nil
	case "clickhouse":
		return storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
			Addr:     strings.Split(options.ClickHouseAddr, ","),
			Database: options.ClickHouseDatabase,
			Username: options.ClickHouseUser,
			Password: options.ClickHousePassword,
		})
	default:
		return nil, fmt.Errorf("unsupported sink %q", options.SinkKind)
	}
}

func env(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func exitIfError(err error) {
	if err == nil || err == context.Canceled {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
