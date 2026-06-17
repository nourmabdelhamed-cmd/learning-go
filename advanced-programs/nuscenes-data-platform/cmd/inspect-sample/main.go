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
	sampleID := flag.String("sample", "", "optional nuScenes sample token")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	output, err := workflow.InspectSample(cfg, *sampleID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(output)
}
