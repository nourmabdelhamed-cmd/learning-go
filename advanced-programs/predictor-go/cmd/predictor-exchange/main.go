package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/metrics"
)

func main() {
	exchange := flag.String("exchange", "", "exchange name for the future live collector")
	metricsAddr := flag.String("metrics-addr", ":8081", "health and metrics address")
	flag.Parse()
	if *exchange == "" {
		fmt.Fprintln(os.Stderr, "error: -exchange is required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metricSet := metrics.New()
	fmt.Printf("predictor-go exchange collector for %s is scaffolded; live adapters are implemented after sample replay\n", *exchange)
	exitIfError(metrics.Serve(ctx, *metricsAddr, metricSet.Registry))
}

func exitIfError(err error) {
	if err == nil || err == context.Canceled {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
