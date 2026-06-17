package nuscenes

import (
	"path/filepath"
	"testing"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
)

func fixtureConfig() config.Config {
	return config.Config{
		DatasetVersion:  "fixture",
		SchemaVersion:   "manifest.v1",
		RawRoot:         filepath.Join("..", "..", "tests", "fixtures", "nuscenes-mini-tiny"),
		MetadataDir:     filepath.Join("..", "..", "tests", "fixtures", "nuscenes-mini-tiny", "v1.0-mini"),
		CanBusDir:       filepath.Join("..", "..", "tests", "fixtures", "nuscenes-mini-tiny", "can_bus"),
		MapExpansionDir: filepath.Join("..", "..", "tests", "fixtures", "nuscenes-mini-tiny", "map-expansion"),
		IncludeCANBus:   true,
		IncludeMaps:     true,
	}
}

func TestLoadBundlesRequireAllSensors(t *testing.T) {
	cfg := fixtureConfig()
	ds, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := ds.Bundles(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 1 {
		t.Fatalf("bundles = %d, want 1", len(bundles))
	}
	bundle := bundles[0]
	if bundle.LiDAR.SensorChannel != "LIDAR_TOP" || len(bundle.Cameras) != 6 {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}
}

func TestCANRowsJoinNearestTimestamp(t *testing.T) {
	cfg := fixtureConfig()
	ds, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := ds.Bundles(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := CANRows(cfg, bundles)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("can rows = %d, want 1", len(rows))
	}
	if rows[0].OdomSpeed != 12 || rows[0].YawRate != 0.03 {
		t.Fatalf("unexpected CAN row: %#v", rows[0])
	}
}

func TestSampleDataRowsCoverAllSensorFiles(t *testing.T) {
	cfg := fixtureConfig()
	ds, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ds.SampleDataRows(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(ds.SampleData) {
		t.Fatalf("sample data rows = %d, want %d", len(rows), len(ds.SampleData))
	}
	if rows[0].Path == "" || rows[0].SizeBytes == 0 {
		t.Fatalf("sample data row missing path/size: %#v", rows[0])
	}
}

func TestMetadataRecordRowsCoverJSONTables(t *testing.T) {
	cfg := fixtureConfig()
	rows, err := MetadataRecordRows(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected metadata records")
	}
	tables := map[string]bool{}
	for _, row := range rows {
		tables[row.TableName] = true
		if row.RecordJSON == "" || row.RecordSHA256 == "" {
			t.Fatalf("metadata record missing json/hash: %#v", row)
		}
	}
	if !tables["sample_data"] || !tables["scene"] {
		t.Fatalf("expected sample_data and scene records, got %#v", tables)
	}
}
