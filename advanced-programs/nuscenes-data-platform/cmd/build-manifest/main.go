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
	rows, err := workflow.BuildManifest(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("manifest=%s rows=%d\n", cfg.ManifestPath, len(rows))
	if len(rows) > 0 {
		fmt.Printf("first_sample_id=%s\n", rows[0].SampleID)
	}
}
