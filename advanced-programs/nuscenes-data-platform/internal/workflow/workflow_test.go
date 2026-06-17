package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/events"
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
	expectedEvents := summary.Scenes + summary.Samples + summary.SensorFiles + summary.CANRows + summary.MapRows + summary.RawAssets + summary.LiDARFiles
	if summary.Events != expectedEvents {
		t.Fatalf("events = %d, want %d from summary %#v", summary.Events, expectedEvents, summary)
	}
	eventRows, err := parquetwriter.Read[events.Envelope](cfg.ParquetPath("events", "events.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	if len(eventRows) != summary.Events {
		t.Fatalf("event parquet rows = %d, want %d", len(eventRows), summary.Events)
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
	manifestSensors := []struct {
		name    string
		payload []byte
		sha     string
		size    int64
	}{
		{"lidar", row.LiDARBytes, row.LiDARSHA256, row.LiDARSizeBytes},
		{"cam_front", row.CamFrontBytes, row.CamFrontSHA256, row.CamFrontSizeBytes},
		{"cam_front_left", row.CamFrontLeftBytes, row.CamFrontLeftSHA256, row.CamFrontLeftSizeBytes},
		{"cam_front_right", row.CamFrontRightBytes, row.CamFrontRightSHA256, row.CamFrontRightSizeBytes},
		{"cam_back", row.CamBackBytes, row.CamBackSHA256, row.CamBackSizeBytes},
		{"cam_back_left", row.CamBackLeftBytes, row.CamBackLeftSHA256, row.CamBackLeftSizeBytes},
		{"cam_back_right", row.CamBackRightBytes, row.CamBackRightSHA256, row.CamBackRightSizeBytes},
	}
	for _, sensor := range manifestSensors {
		if len(sensor.payload) == 0 {
			t.Fatalf("parquet manifest missing %s bytes", sensor.name)
		}
		if int64(len(sensor.payload)) != sensor.size {
			t.Fatalf("%s size = %d, want %d", sensor.name, len(sensor.payload), sensor.size)
		}
		if got := digest(sensor.payload); got != sensor.sha {
			t.Fatalf("%s sha mismatch: got %s, want %s", sensor.name, got, sensor.sha)
		}
	}
}

func TestManifestBuildersRequireUpstreamArtifacts(t *testing.T) {
	cfg := fixtureConfig(t)
	expected := "required upstream artifact missing: " + cfg.ParquetPath("metadata", "samples.parquet")

	if _, err := BuildManifest(cfg); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("BuildManifest error = %v, want %q", err, expected)
	}
	if _, err := BuildParquetManifest(cfg); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("BuildParquetManifest error = %v, want %q", err, expected)
	}
}

func TestPublishConcurrentlyRunsPublishersInGoroutines(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	publishers := []func() error{
		func() error {
			started <- struct{}{}
			<-release
			return nil
		},
		func() error {
			started <- struct{}{}
			<-release
			return nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- publishConcurrently(publishers)
	}()

	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	for i := 0; i < len(publishers); i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("publisher %d did not start concurrently", i)
		}
	}
	close(release)
	released = true
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("publishers did not finish")
	}
}

func digest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
