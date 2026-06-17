package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/manifest"
)

func main() {
	configPath := flag.String("config", "configs/btc-instruments.sample.json", "instrument manifest JSON file")
	redisURL := flag.String("redis-url", "", "Redis URL; if empty, print manifest instead of publishing")
	key := flag.String("key", contract.InstrumentManifestKey, "Redis key for instrument manifest")
	timeout := flag.Duration("timeout", 10*time.Second, "command timeout")
	flag.Parse()

	exitIfError(run(*configPath, *redisURL, *key, *timeout))
}

func run(configPath, redisURL, key string, timeout time.Duration) error {
	loaded, err := manifest.Load(configPath)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(loaded)
	if err != nil {
		return err
	}
	if redisURL == "" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(loaded)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return err
	}
	client := redis.NewClient(options)
	defer client.Close()
	if err := client.Set(ctx, key, payload, 0).Err(); err != nil {
		return err
	}
	fmt.Printf("published %d instruments to %s\n", len(loaded.Instruments), key)
	return nil
}

func exitIfError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
