package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateTelemetryRejectsImpossibleValues(t *testing.T) {
	records := []TelemetryRecord{
		{
			Timestamp:       time.Now().UTC(),
			VehicleID:       "av-001",
			TripID:          "trip-001",
			SpeedKPH:        42,
			BrakePressure:   0.5,
			WheelVibration:  0.2,
			TirePressurePSI: 32,
			BatteryVoltage:  12.4,
			CameraTempC:     35,
			LidarTempC:      35,
			GPSAccuracyM:    2,
			RoadFriction:    0.9,
		},
		{
			Timestamp:       time.Now().UTC(),
			VehicleID:       "av-002",
			TripID:          "trip-001",
			SpeedKPH:        -3,
			BrakePressure:   1.4,
			WheelVibration:  0.2,
			TirePressurePSI: 32,
			BatteryVoltage:  12.4,
			CameraTempC:     35,
			LidarTempC:      35,
			GPSAccuracyM:    2,
			RoadFriction:    0.9,
		},
	}

	result := ValidateTelemetry(records)
	if result.Report.ValidRecords != 1 {
		t.Fatalf("valid records = %d, want 1", result.Report.ValidRecords)
	}
	if result.Report.IssueCounts["speed_out_of_range"] != 1 {
		t.Fatalf("missing speed issue count: %#v", result.Report.IssueCounts)
	}
	if result.Report.IssueCounts["brake_pressure_out_of_range"] != 1 {
		t.Fatalf("missing brake issue count: %#v", result.Report.IssueCounts)
	}
}

func TestFeatureLabelsCoverPortfolioUseCases(t *testing.T) {
	cfg := DefaultConfig()
	record := TelemetryRecord{
		Timestamp:        time.Now().UTC(),
		VehicleID:        "av-001",
		TripID:           "trip-001",
		SpeedKPH:         72,
		AccelXMPS2:       -7,
		AccelYMPS2:       5,
		YawRateDPS:       45,
		SteeringAngleDeg: 25,
		BrakePressure:    0.9,
		WheelVibration:   0.82,
		TirePressurePSI:  28,
		BatteryVoltage:   11.4,
		CameraTempC:      48,
		LidarTempC:       35,
		GPSAccuracyM:     30,
		RoadFriction:     0.35,
	}

	features := BuildFeatures([]TelemetryRecord{record}, cfg.Thresholds)
	if len(features) != 1 {
		t.Fatalf("features length = %d, want 1", len(features))
	}
	feature := features[0]
	assertTrue(t, feature.SensorAnomaly, "sensor anomaly")
	assertTrue(t, feature.HarshBraking, "harsh braking")
	assertTrue(t, feature.UnsafeManeuver, "unsafe maneuver")
	assertTrue(t, feature.SensorDrift, "sensor drift")
	assertTrue(t, feature.VehicleHealthIssue, "vehicle health")
	if feature.RoadCondition != "icy" {
		t.Fatalf("road condition = %q, want icy", feature.RoadCondition)
	}
}

func TestRunAllWritesDataEngineeringArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Rows = 420
	cfg.DataDir = filepath.Join(tempDir, "data")

	result, err := RunAll(cfg)
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	for _, path := range []string{
		result.Paths.BronzeTelemetryCSV,
		result.Paths.SilverTelemetryCSV,
		result.Paths.GoldFeaturesCSV,
		result.Paths.GoldEventsCSV,
		result.Paths.MonitoringReportJSON,
		result.Paths.QualityReportJSON,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}
}

func TestReadTelemetryCSVReportsLineNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.csv")
	payload := strings.Join([]string{
		strings.Join(telemetryHeader, ","),
		"2026-01-05T08:00:00Z,av-001,trip-001,50,0,0,0,0,0.2,0.1,32,12.4,35,35,2,0.9",
		"2026-01-05T08:00:01Z,av-001,trip-001,not-a-number,0,0,0,0,0.2,0.1,32,12.4,35,35,2,0.9",
	}, "\n")
	if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
		t.Fatalf("write csv fixture: %v", err)
	}

	_, err := ReadTelemetryCSV(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("error = %q, want line number", err.Error())
	}
}

func TestReadFeatureCSVParsesGeneratedFeatureFile(t *testing.T) {
	cfg := DefaultConfig()
	records := GenerateTelemetry(40, 2, cfg.Seed)
	features := BuildFeatures(records, cfg.Thresholds)
	path := filepath.Join(t.TempDir(), "features.csv")

	if err := WriteFeatureCSV(path, features); err != nil {
		t.Fatalf("write features: %v", err)
	}
	read, err := ReadFeatureCSV(path)
	if err != nil {
		t.Fatalf("read features: %v", err)
	}
	if len(read) != len(features) {
		t.Fatalf("read %d features, want %d", len(read), len(features))
	}
	if read[0].VehicleID == "" {
		t.Fatal("expected vehicle id to round-trip")
	}
}

func assertTrue(t *testing.T, value bool, name string) {
	t.Helper()
	if !value {
		t.Fatalf("expected %s to be true", name)
	}
}
