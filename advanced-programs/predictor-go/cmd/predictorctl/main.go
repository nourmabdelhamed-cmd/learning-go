package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/app"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/broker"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate-samples":
		exitIfError(runValidateSamples(os.Args[2:]))
	case "replay-samples":
		exitIfError(runReplaySamples(os.Args[2:]))
	default:
		printUsage()
		os.Exit(2)
	}
}

func runValidateSamples(args []string) error {
	flags := flag.NewFlagSet("validate-samples", flag.ExitOnError)
	samplesDir := flags.String("samples-dir", "../predictor/samples", "directory containing BTC sample JSON files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	report, err := app.ValidateSamples(*samplesDir)
	if err != nil {
		return err
	}
	return printJSON(report)
}

func runReplaySamples(args []string) error {
	flags := flag.NewFlagSet("replay-samples", flag.ExitOnError)
	samplesDir := flags.String("samples-dir", "../predictor/samples", "directory containing BTC sample JSON files")
	sink := flags.String("sink", "stdout", "replay sink: stdout or redis")
	redisURL := flags.String("redis-url", "redis://localhost:6379/0", "Redis URL when -sink=redis")
	rawStream := flags.String("raw-stream", contract.RawRedisStream, "Redis stream for raw payloads")
	normalizedStream := flags.String("normalized-stream", contract.NormalizedRedisStream, "Redis stream for normalized order books")
	timeout := flags.Duration("timeout", 10*time.Second, "command timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var publisher broker.Publisher
	memory := &broker.MemoryPublisher{}
	switch *sink {
	case "stdout":
		publisher = memory
	case "redis":
		options, err := redis.ParseURL(*redisURL)
		if err != nil {
			return err
		}
		client := redis.NewClient(options)
		defer client.Close()
		publisher = broker.NewRedisPublisher(client, *rawStream, *normalizedStream, 0)
	default:
		return fmt.Errorf("unsupported sink %q", *sink)
	}

	result, err := app.ReplaySamples(ctx, *samplesDir, publisher)
	if err != nil {
		return err
	}
	if *sink == "stdout" {
		result.PublishedRaw = len(memory.Raw)
		result.PublishedNormalized = len(memory.Normalized)
	}
	return printJSON(result)
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printUsage() {
	fmt.Println("usage: predictorctl <validate-samples|replay-samples> [flags]")
	fmt.Println()
	fmt.Println("examples:")
	fmt.Println("  go run ./cmd/predictorctl validate-samples")
	fmt.Println("  go run ./cmd/predictorctl replay-samples -sink stdout")
	fmt.Println("  go run ./cmd/predictorctl replay-samples -sink redis -redis-url redis://localhost:6379/0")
}

func exitIfError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
