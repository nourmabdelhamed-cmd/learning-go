package workflow

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/nuscenes"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/parquetwriter"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/rawassets"
)

const ParquetManifestSchemaVersion = "manifest.v2"

func BuildParquetManifest(cfg config.Config) (ParquetManifestSummary, error) {
	if err := requireArtifacts(
		cfg.ParquetPath("metadata", "samples.parquet"),
		cfg.ParquetPath("metadata", "sample_sensors.parquet"),
		cfg.ParquetPath("metadata", "calibrations.parquet"),
		cfg.ParquetPath("metadata", "ego_poses.parquet"),
		cfg.ParquetPath("bronze", "raw_assets.parquet"),
		cfg.ParquetPath("bronze", "raw_asset_chunks"),
	); err != nil {
		return ParquetManifestSummary{}, err
	}

	samples, err := parquetwriter.Read[nuscenes.SampleRow](cfg.ParquetPath("metadata", "samples.parquet"))
	if err != nil {
		return ParquetManifestSummary{}, err
	}
	sensors, err := parquetwriter.Read[nuscenes.SampleSensorRow](cfg.ParquetPath("metadata", "sample_sensors.parquet"))
	if err != nil {
		return ParquetManifestSummary{}, err
	}
	calibrations, err := parquetwriter.Read[nuscenes.CalibrationRow](cfg.ParquetPath("metadata", "calibrations.parquet"))
	if err != nil {
		return ParquetManifestSummary{}, err
	}
	egoPoses, err := parquetwriter.Read[nuscenes.EgoPoseRow](cfg.ParquetPath("metadata", "ego_poses.parquet"))
	if err != nil {
		return ParquetManifestSummary{}, err
	}
	assets, err := parquetwriter.Read[rawassets.AssetRow](cfg.ParquetPath("bronze", "raw_assets.parquet"))
	if err != nil {
		return ParquetManifestSummary{}, err
	}

	sort.Slice(samples, func(i, j int) bool {
		if samples[i].SceneID == samples[j].SceneID {
			return samples[i].Timestamp < samples[j].Timestamp
		}
		return samples[i].SceneID < samples[j].SceneID
	})
	bySample := sampleSensorsBySample(sensors)
	assetsByPath := assetsByCleanPath(assets)
	requiredAssets, err := collectParquetManifestAssets(samples, bySample, assetsByPath)
	if err != nil {
		return ParquetManifestSummary{}, err
	}
	assetBytes, err := rawassets.ReadSelectedAssetBytes(cfg.ParquetPath("bronze", "raw_asset_chunks"), requiredAssets)
	if err != nil {
		return ParquetManifestSummary{}, err
	}

	calibrationVersion := nuscenes.VersionHash(calibrations)
	transformGraphVersion := nuscenes.VersionHash(map[string]any{
		"calibration_version": calibrationVersion,
		"ego_poses":           egoPoses,
	})

	writer, err := parquetwriter.NewStream[ParquetManifestRow](cfg.ParquetManifestPath)
	if err != nil {
		return ParquetManifestSummary{}, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = writer.Close()
		}
	}()

	summary := ParquetManifestSummary{Path: cfg.ParquetManifestPath}
	for _, sample := range samples {
		row, err := parquetManifestRow(cfg, sample, bySample[sample.SampleID], assetsByPath, assetBytes, calibrationVersion, transformGraphVersion)
		if err != nil {
			return ParquetManifestSummary{}, err
		}
		if err := writer.Write([]ParquetManifestRow{row}); err != nil {
			return ParquetManifestSummary{}, err
		}
		if summary.Rows == 0 {
			summary.FirstSampleID = row.SampleID
		}
		summary.Rows++
	}
	if err := writer.Close(); err != nil {
		return ParquetManifestSummary{}, err
	}
	closed = true
	return summary, nil
}

func sampleSensorsBySample(sensors []nuscenes.SampleSensorRow) map[string]map[string]nuscenes.SampleSensorRow {
	bySample := map[string]map[string]nuscenes.SampleSensorRow{}
	for _, sensor := range sensors {
		if _, ok := bySample[sensor.SampleID]; !ok {
			bySample[sensor.SampleID] = map[string]nuscenes.SampleSensorRow{}
		}
		bySample[sensor.SampleID][sensor.SensorChannel] = sensor
	}
	return bySample
}

func assetsByCleanPath(assets []rawassets.AssetRow) map[string]rawassets.AssetRow {
	byPath := map[string]rawassets.AssetRow{}
	for _, asset := range assets {
		byPath[filepath.Clean(asset.Path)] = asset
	}
	return byPath
}

func collectParquetManifestAssets(samples []nuscenes.SampleRow, bySample map[string]map[string]nuscenes.SampleSensorRow, assetsByPath map[string]rawassets.AssetRow) (map[string]rawassets.AssetRow, error) {
	required := map[string]rawassets.AssetRow{}
	for _, sample := range samples {
		sampleSensors := bySample[sample.SampleID]
		for _, spec := range parquetManifestSensorSpecs {
			sensor, err := requireParquetManifestSensor(sample.SampleID, sampleSensors, spec)
			if err != nil {
				return nil, err
			}
			asset, err := assetForSensor(sample.SampleID, sensor, assetsByPath)
			if err != nil {
				return nil, err
			}
			required[asset.AssetID] = asset
		}
	}
	return required, nil
}

func parquetManifestChannels() []string {
	channels := make([]string, 0, len(parquetManifestSensorSpecs))
	for _, spec := range parquetManifestSensorSpecs {
		channels = append(channels, spec.channel)
	}
	return channels
}

func requireParquetManifestSensor(sampleID string, sampleSensors map[string]nuscenes.SampleSensorRow, spec parquetManifestSensorSpec) (nuscenes.SampleSensorRow, error) {
	sensor, ok := sampleSensors[spec.channel]
	if !ok {
		return nuscenes.SampleSensorRow{}, fmt.Errorf("sample %s missing %s in Parquet metadata", sampleID, spec.channel)
	}
	return sensor, nil
}

func assetForSensor(sampleID string, sensor nuscenes.SampleSensorRow, assetsByPath map[string]rawassets.AssetRow) (rawassets.AssetRow, error) {
	asset, ok := assetsByPath[filepath.Clean(sensor.Path)]
	if !ok {
		return rawassets.AssetRow{}, fmt.Errorf("sample %s %s missing from raw asset inventory: %s", sampleID, sensor.SensorChannel, sensor.Path)
	}
	return asset, nil
}

func parquetManifestRow(
	cfg config.Config,
	sample nuscenes.SampleRow,
	sampleSensors map[string]nuscenes.SampleSensorRow,
	assetsByPath map[string]rawassets.AssetRow,
	assetBytes map[string][]byte,
	calibrationVersion string,
	transformGraphVersion string,
) (ParquetManifestRow, error) {
	row := ParquetManifestRow{
		SampleID:              sample.SampleID,
		SceneID:               sample.SceneID,
		Timestamp:             sample.Timestamp,
		DatasetVersion:        cfg.DatasetVersion,
		SchemaVersion:         ParquetManifestSchemaVersion,
		CalibrationVersion:    calibrationVersion,
		TransformGraphVersion: transformGraphVersion,
	}
	for _, spec := range parquetManifestSensorSpecs {
		if err := applyParquetManifestSensor(&row, sample.SampleID, sampleSensors, assetsByPath, assetBytes, spec); err != nil {
			return ParquetManifestRow{}, err
		}
	}
	return row, nil
}

type parquetManifestSensorSpec struct {
	channel string
	set     func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte)
}

var parquetManifestSensorSpecs = []parquetManifestSensorSpec{
	{
		channel: "LIDAR_TOP",
		set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
			row.LiDARAssetID = asset.AssetID
			row.LiDARRelativePath = asset.RelativePath
			row.LiDARPath = asset.Path
			row.LiDARURI = asset.URI
			row.LiDARSHA256 = asset.SHA256
			row.LiDARSizeBytes = asset.SizeBytes
			row.LiDARMediaType = asset.MediaType
			row.LiDARChunkCount = asset.ChunkCount
			row.LiDARBytes = payload
		},
	},
	{
		channel: "CAM_FRONT",
		set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
			row.CamFrontAssetID = asset.AssetID
			row.CamFrontRelativePath = asset.RelativePath
			row.CamFrontPath = asset.Path
			row.CamFrontURI = asset.URI
			row.CamFrontSHA256 = asset.SHA256
			row.CamFrontSizeBytes = asset.SizeBytes
			row.CamFrontMediaType = asset.MediaType
			row.CamFrontChunkCount = asset.ChunkCount
			row.CamFrontBytes = payload
		},
	},
	{
		channel: "CAM_FRONT_LEFT",
		set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
			row.CamFrontLeftAssetID = asset.AssetID
			row.CamFrontLeftRelativePath = asset.RelativePath
			row.CamFrontLeftPath = asset.Path
			row.CamFrontLeftURI = asset.URI
			row.CamFrontLeftSHA256 = asset.SHA256
			row.CamFrontLeftSizeBytes = asset.SizeBytes
			row.CamFrontLeftMediaType = asset.MediaType
			row.CamFrontLeftChunkCount = asset.ChunkCount
			row.CamFrontLeftBytes = payload
		},
	},
	{
		channel: "CAM_FRONT_RIGHT",
		set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
			row.CamFrontRightAssetID = asset.AssetID
			row.CamFrontRightRelativePath = asset.RelativePath
			row.CamFrontRightPath = asset.Path
			row.CamFrontRightURI = asset.URI
			row.CamFrontRightSHA256 = asset.SHA256
			row.CamFrontRightSizeBytes = asset.SizeBytes
			row.CamFrontRightMediaType = asset.MediaType
			row.CamFrontRightChunkCount = asset.ChunkCount
			row.CamFrontRightBytes = payload
		},
	},
	{
		channel: "CAM_BACK",
		set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
			row.CamBackAssetID = asset.AssetID
			row.CamBackRelativePath = asset.RelativePath
			row.CamBackPath = asset.Path
			row.CamBackURI = asset.URI
			row.CamBackSHA256 = asset.SHA256
			row.CamBackSizeBytes = asset.SizeBytes
			row.CamBackMediaType = asset.MediaType
			row.CamBackChunkCount = asset.ChunkCount
			row.CamBackBytes = payload
		},
	},
	{
		channel: "CAM_BACK_LEFT",
		set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
			row.CamBackLeftAssetID = asset.AssetID
			row.CamBackLeftRelativePath = asset.RelativePath
			row.CamBackLeftPath = asset.Path
			row.CamBackLeftURI = asset.URI
			row.CamBackLeftSHA256 = asset.SHA256
			row.CamBackLeftSizeBytes = asset.SizeBytes
			row.CamBackLeftMediaType = asset.MediaType
			row.CamBackLeftChunkCount = asset.ChunkCount
			row.CamBackLeftBytes = payload
		},
	},
	{
		channel: "CAM_BACK_RIGHT",
		set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
			row.CamBackRightAssetID = asset.AssetID
			row.CamBackRightRelativePath = asset.RelativePath
			row.CamBackRightPath = asset.Path
			row.CamBackRightURI = asset.URI
			row.CamBackRightSHA256 = asset.SHA256
			row.CamBackRightSizeBytes = asset.SizeBytes
			row.CamBackRightMediaType = asset.MediaType
			row.CamBackRightChunkCount = asset.ChunkCount
			row.CamBackRightBytes = payload
		},
	},
}

func applyParquetManifestSensor(
	row *ParquetManifestRow,
	sampleID string,
	sampleSensors map[string]nuscenes.SampleSensorRow,
	assetsByPath map[string]rawassets.AssetRow,
	assetBytes map[string][]byte,
	spec parquetManifestSensorSpec,
) error {
	sensor, err := requireParquetManifestSensor(sampleID, sampleSensors, spec)
	if err != nil {
		return err
	}
	asset, err := assetForSensor(sampleID, sensor, assetsByPath)
	if err != nil {
		return err
	}
	payload := assetBytes[asset.AssetID]
	if err := rawassets.VerifyAssetBytes(asset, payload); err != nil {
		return err
	}
	if spec.channel == "LIDAR_TOP" {
		row.EgoPoseID = sensor.EgoPoseID
		row.CalibrationID = sensor.CalibrationID
	}
	spec.set(row, asset, payload)
	return nil
}
