package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/workflow"
)

func main() {
	configPath := flag.String("config", "configs/nuscenes-mini.local.json", "config JSON path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	summary, err := workflow.BuildParquetManifest(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("parquet_manifest=%s rows=%d\n", summary.Path, summary.Rows)
	if summary.FirstSampleID != "" {
		fmt.Printf("first_sample_id=%s\n", summary.FirstSampleID)
	}
}
