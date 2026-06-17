package main

import (
	"context"
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
	summary, err := workflow.Ingest(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("scenes=%d samples=%d sensor_files=%d sample_data_rows=%d metadata_records=%d can_rows=%d map_rows=%d lidar_files=%d lidar_points=%d raw_assets=%d raw_asset_chunks=%d raw_asset_bytes=%d events=%d\n",
		summary.Scenes,
		summary.Samples,
		summary.SensorFiles,
		summary.SampleDataRows,
		summary.MetadataRecords,
		summary.CANRows,
		summary.MapRows,
		summary.LiDARFiles,
		summary.LiDARPoints,
		summary.RawAssets,
		summary.RawAssetChunks,
		summary.RawAssetBytes,
		summary.Events,
	)
}
