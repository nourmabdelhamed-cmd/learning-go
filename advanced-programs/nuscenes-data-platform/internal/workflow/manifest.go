package workflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/events"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/nuscenes"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/parquetwriter"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/rawassets"
)

func BuildManifest(cfg config.Config) ([]ManifestRow, error) {
	if err := requireArtifacts(
		cfg.ParquetPath("metadata", "samples.parquet"),
		cfg.ParquetPath("metadata", "sample_sensors.parquet"),
		cfg.ParquetPath("metadata", "calibrations.parquet"),
		cfg.ParquetPath("metadata", "ego_poses.parquet"),
		cfg.ParquetPath("bronze", "raw_assets.parquet"),
	); err != nil {
		return nil, err
	}
	samples, err := parquetwriter.Read[nuscenes.SampleRow](cfg.ParquetPath("metadata", "samples.parquet"))
	if err != nil {
		return nil, err
	}
	sensors, err := parquetwriter.Read[nuscenes.SampleSensorRow](cfg.ParquetPath("metadata", "sample_sensors.parquet"))
	if err != nil {
		return nil, err
	}
	calibrations, err := parquetwriter.Read[nuscenes.CalibrationRow](cfg.ParquetPath("metadata", "calibrations.parquet"))
	if err != nil {
		return nil, err
	}
	egoPoses, err := parquetwriter.Read[nuscenes.EgoPoseRow](cfg.ParquetPath("metadata", "ego_poses.parquet"))
	if err != nil {
		return nil, err
	}
	assets, err := parquetwriter.Read[rawassets.AssetRow](cfg.ParquetPath("bronze", "raw_assets.parquet"))
	if err != nil {
		return nil, err
	}
	assetPaths := map[string]bool{}
	for _, asset := range assets {
		assetPaths[filepath.Clean(asset.Path)] = true
	}

	bySample := map[string]map[string]nuscenes.SampleSensorRow{}
	for _, sensor := range sensors {
		if _, ok := bySample[sensor.SampleID]; !ok {
			bySample[sensor.SampleID] = map[string]nuscenes.SampleSensorRow{}
		}
		bySample[sensor.SampleID][sensor.SensorChannel] = sensor
	}

	calibrationVersion := nuscenes.VersionHash(calibrations)
	transformGraphVersion := nuscenes.VersionHash(map[string]any{
		"calibration_version": calibrationVersion,
		"ego_poses":           egoPoses,
	})

	rows := make([]ManifestRow, 0, len(samples))
	for _, sample := range samples {
		sampleSensors := bySample[sample.SampleID]
		lidar, ok := sampleSensors["LIDAR_TOP"]
		if !ok {
			return nil, fmt.Errorf("sample %s missing LIDAR_TOP in Parquet metadata", sample.SampleID)
		}
		if !assetPaths[filepath.Clean(lidar.Path)] {
			return nil, fmt.Errorf("sample %s LIDAR_TOP missing from raw asset inventory: %s", sample.SampleID, lidar.Path)
		}
		row := ManifestRow{
			SampleID:              sample.SampleID,
			SceneID:               sample.SceneID,
			Timestamp:             sample.Timestamp,
			LiDARPath:             lidar.Path,
			EgoPoseID:             lidar.EgoPoseID,
			CalibrationID:         lidar.CalibrationID,
			DatasetVersion:        cfg.DatasetVersion,
			SchemaVersion:         cfg.SchemaVersion,
			CalibrationVersion:    calibrationVersion,
			TransformGraphVersion: transformGraphVersion,
		}
		for _, channel := range nuscenes.RequiredCameraChannels {
			sensor, ok := sampleSensors[channel]
			if !ok {
				return nil, fmt.Errorf("sample %s missing %s in Parquet metadata", sample.SampleID, channel)
			}
			if !assetPaths[filepath.Clean(sensor.Path)] {
				return nil, fmt.Errorf("sample %s %s missing from raw asset inventory: %s", sample.SampleID, channel, sensor.Path)
			}
			switch channel {
			case "CAM_FRONT":
				row.CamFrontPath = sensor.Path
			case "CAM_FRONT_LEFT":
				row.CamFrontLeftPath = sensor.Path
			case "CAM_FRONT_RIGHT":
				row.CamFrontRightPath = sensor.Path
			case "CAM_BACK":
				row.CamBackPath = sensor.Path
			case "CAM_BACK_LEFT":
				row.CamBackLeftPath = sensor.Path
			case "CAM_BACK_RIGHT":
				row.CamBackRightPath = sensor.Path
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SceneID == rows[j].SceneID {
			return rows[i].Timestamp < rows[j].Timestamp
		}
		return rows[i].SceneID < rows[j].SceneID
	})
	if err := writeManifestJSONL(cfg.ManifestPath, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func writeManifestJSONL(path string, rows []ManifestRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, row := range rows {
		payload, err := events.Payload(row)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(payload, '\n')); err != nil {
			return err
		}
	}
	return writer.Flush()
}
