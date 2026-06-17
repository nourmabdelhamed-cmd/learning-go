package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/lidar"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/parquetwriter"
)

func fixtureConfig(t *testing.T) config.Config {
	root := filepath.Join("..", "..", "tests", "fixtures", "nuscenes-mini-tiny")
	runRoot := t.TempDir()
	return config.Config{
		DatasetVersion:          "fixture",
		SchemaVersion:           "manifest.v1",
		RawRoot:                 root,
		RawAssetRoots:           []string{root},
		MetadataDir:             filepath.Join(root, "v1.0-mini"),
		SamplesDir:              filepath.Join(root, "samples"),
		MapsDir:                 filepath.Join(root, "maps"),
		CanBusDir:               filepath.Join(root, "can_bus"),
		MapExpansionDir:         filepath.Join(root, "map-expansion"),
		LakeDir:                 filepath.Join(runRoot, "lake"),
		EventLogPath:            filepath.Join(runRoot, "events", "events.jsonl"),
		ManifestPath:            filepath.Join(runRoot, "manifests", "training_manifest.jsonl"),
		ParquetManifestPath:     filepath.Join(runRoot, "lake", "training", "training_manifest_v2.parquet"),
		IncludeCANBus:           true,
		IncludeMaps:             true,
		IncludeRawAssets:        true,
		RawAssetChunkBytes:      64,
		MaxLiDARSamples:         1,
		MaxLiDARPointsPerSample: 16,
	}
}

func TestIngestAndBuildManifest(t *testing.T) {
	cfg := fixtureConfig(t)
	summary, err := Ingest(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Samples != 1 || summary.SensorFiles != 7 || summary.LiDARPoints == 0 || summary.RawAssets == 0 || summary.RawAssetBytes == 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	rows, err := BuildManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("manifest rows = %d, want 1", len(rows))
	}
	if rows[0].CamFrontPath == "" || rows[0].LiDARPath == "" {
		t.Fatalf("manifest missing paths: %#v", rows[0])
	}

	parquetSummary, err := BuildParquetManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if parquetSummary.Rows != 1 || parquetSummary.FirstSampleID == "" {
		t.Fatalf("unexpected parquet manifest summary: %#v", parquetSummary)
	}
	parquetRows, err := parquetwriter.Read[ParquetManifestRow](cfg.ParquetManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(parquetRows) != 1 {
		t.Fatalf("parquet manifest rows = %d, want 1", len(parquetRows))
	}
	row := parquetRows[0]
	if row.SchemaVersion != ParquetManifestSchemaVersion {
		t.Fatalf("schema version = %s, want %s", row.SchemaVersion, ParquetManifestSchemaVersion)
	}
	if row.LiDARAssetID == "" || row.CamFrontAssetID == "" {
		t.Fatalf("parquet manifest missing asset ids: %#v", row)
	}
	lidarRows, err := parquetwriter.Read[LiDARPointRow](cfg.ParquetPath("features", "lidar_points.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lidarRows) != summary.LiDARPoints {
		t.Fatalf("lidar point rows = %d, want %d", len(lidarRows), summary.LiDARPoints)
	}
	lidarPoint := lidarRows[0]
	if lidarPoint.LiDARAssetID != row.LiDARAssetID || lidarPoint.LiDARSHA256 != row.LiDARSHA256 {
		t.Fatalf("lidar point provenance does not match manifest asset: point=%#v manifest=%#v", lidarPoint, row)
	}
	if lidarPoint.ParserVersion != lidar.ParserVersion {
		t.Fatalf("lidar parser version = %s, want %s", lidarPoint.ParserVersion, lidar.ParserVersion)
	}
	if lidarPoint.SourcePointCount < lidarPoint.DecodedPointCount || lidarPoint.DecodedPointCount != summary.LiDARPoints {
		t.Fatalf("unexpected lidar point counts: point=%#v summary=%#v", lidarPoint, summary)
	}
	if len(row.LiDARBytes) == 0 || len(row.CamFrontBytes) == 0 {
		t.Fatalf("parquet manifest missing sensor bytes")
	}
	if got := digest(row.LiDARBytes); got != row.LiDARSHA256 {
		t.Fatalf("lidar sha mismatch: got %s, want %s", got, row.LiDARSHA256)
	}
	if got := digest(row.CamFrontBytes); got != row.CamFrontSHA256 {
		t.Fatalf("camera sha mismatch: got %s, want %s", got, row.CamFrontSHA256)
	}
}

func digest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
