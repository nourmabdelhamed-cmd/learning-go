package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/events"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/lidar"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/nuscenes"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/parquetwriter"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/rawassets"
)

func Ingest(ctx context.Context, cfg config.Config) (IngestSummary, error) {
	if err := cfg.RequireRawData(); err != nil {
		return IngestSummary{}, err
	}
	ds, err := nuscenes.Load(cfg)
	if err != nil {
		return IngestSummary{}, err
	}
	bundles, err := ds.Bundles(cfg)
	if err != nil {
		return IngestSummary{}, err
	}
	if len(bundles) == 0 {
		return IngestSummary{}, fmt.Errorf("no valid samples found")
	}

	publisher, err := events.NewJSONLPublisher(cfg.EventLogPath)
	if err != nil {
		return IngestSummary{}, err
	}
	defer publisher.Close()

	sceneRows := filterSceneRows(ds.SceneRows(), bundles)
	sampleRows := nuscenes.SampleRows(bundles)
	sensorRows := nuscenes.SampleSensorRows(bundles)
	sampleDataRows, err := ds.SampleDataRows(cfg)
	if err != nil {
		return IngestSummary{}, err
	}
	metadataRows, err := nuscenes.MetadataRecordRows(cfg)
	if err != nil {
		return IngestSummary{}, err
	}
	egoPoseRows := ds.EgoPoseRows()
	calibrationRows := ds.CalibrationRows()
	annotationRows := ds.AnnotationRows()
	mapRows := ds.MapRows(cfg)
	canRows, err := nuscenes.CANRows(cfg, bundles)
	if err != nil {
		return IngestSummary{}, err
	}
	rawAssetRows, rawAssetSummary, err := rawassets.Write(cfg)
	if err != nil {
		return IngestSummary{}, err
	}

	for _, row := range sceneRows {
		if err := publish(ctx, publisher, "nuscenes.scene.v1", row.SceneID, "scene", cfg, row.SceneID, "", "", "", row); err != nil {
			return IngestSummary{}, err
		}
	}
	for _, row := range sampleRows {
		if err := publish(ctx, publisher, "nuscenes.sample.v1", row.SampleID, "sample", cfg, row.SceneID, row.SampleID, "", "", row); err != nil {
			return IngestSummary{}, err
		}
	}
	for _, row := range sensorRows {
		if err := publish(ctx, publisher, "nuscenes.sensor_file.v1", row.SampleID+":"+row.SensorChannel, "sensor_file", cfg, row.SceneID, row.SampleID, row.SensorChannel, row.Path, row); err != nil {
			return IngestSummary{}, err
		}
	}
	for _, row := range canRows {
		if err := publish(ctx, publisher, "nuscenes.can_bus.v1", row.SampleID, "can_bus", cfg, row.SceneID, row.SampleID, "", "", row); err != nil {
			return IngestSummary{}, err
		}
	}
	for _, row := range mapRows {
		if err := publish(ctx, publisher, "nuscenes.map.v1", row.MapID, "map", cfg, "", "", "", row.Path, row); err != nil {
			return IngestSummary{}, err
		}
	}
	for _, row := range rawAssetRows {
		if err := publish(ctx, publisher, "nuscenes.raw_asset.v1", row.AssetID, "raw_asset", cfg, "", "", "", row.Path, row); err != nil {
			return IngestSummary{}, err
		}
	}
	lidarSummary, err := WriteLiDARPoints(cfg, bundles, rawAssetRows)
	if err != nil {
		return IngestSummary{}, err
	}
	assetsByPath := assetsByCleanPath(rawAssetRows)
	for _, bundle := range lidarBundles(cfg, bundles) {
		asset, err := lidarAssetForBundle(bundle, assetsByPath)
		if err != nil {
			return IngestSummary{}, err
		}
		if err := publish(ctx, publisher, "nuscenes.lidar_points.v1", bundle.SampleID, "lidar_points", cfg, bundle.SceneID, bundle.SampleID, "LIDAR_TOP", bundle.LiDAR.Path, map[string]any{
			"sample_id":           bundle.SampleID,
			"scene_id":            bundle.SceneID,
			"path":                bundle.LiDAR.Path,
			"lidar_asset_id":      asset.AssetID,
			"lidar_relative_path": asset.RelativePath,
			"lidar_sha256":        asset.SHA256,
			"feature_path":        cfg.ParquetPath("features", "lidar_points.parquet"),
			"parser_version":      lidar.ParserVersion,
		}); err != nil {
			return IngestSummary{}, err
		}
	}

	if err := writeTables(cfg, sceneRows, sampleRows, sensorRows, sampleDataRows, metadataRows, egoPoseRows, calibrationRows, annotationRows, mapRows, canRows, publisher.Events()); err != nil {
		return IngestSummary{}, err
	}

	return IngestSummary{
		Scenes:          len(sceneRows),
		Samples:         len(sampleRows),
		SensorFiles:     len(sensorRows),
		SampleDataRows:  len(sampleDataRows),
		MetadataRecords: len(metadataRows),
		CANRows:         len(canRows),
		MapRows:         len(mapRows),
		LiDARFiles:      lidarSummary.Files,
		LiDARPoints:     lidarSummary.Points,
		RawAssets:       rawAssetSummary.Assets,
		RawAssetChunks:  rawAssetSummary.Chunks,
		RawAssetBytes:   rawAssetSummary.Bytes,
		Events:          len(publisher.Events()),
	}, nil
}

func publish(ctx context.Context, publisher events.Publisher, topic string, key string, eventType string, cfg config.Config, sceneID string, sampleID string, sensorChannel string, sourcePath string, payloadValue any) error {
	payload, err := events.Payload(payloadValue)
	if err != nil {
		return err
	}
	return publisher.Publish(ctx, topic, key, events.Envelope{
		EventType:      eventType,
		DatasetVersion: cfg.DatasetVersion,
		SchemaVersion:  cfg.SchemaVersion,
		SceneID:        sceneID,
		SampleID:       sampleID,
		SensorChannel:  sensorChannel,
		SourcePath:     sourcePath,
		Payload:        payload,
	})
}

func writeTables(
	cfg config.Config,
	sceneRows []nuscenes.SceneRow,
	sampleRows []nuscenes.SampleRow,
	sensorRows []nuscenes.SampleSensorRow,
	sampleDataRows []nuscenes.SampleDataRow,
	metadataRows []nuscenes.MetadataRecordRow,
	egoPoseRows []nuscenes.EgoPoseRow,
	calibrationRows []nuscenes.CalibrationRow,
	annotationRows []nuscenes.AnnotationRow,
	mapRows []nuscenes.MapRow,
	canRows []nuscenes.CANBusRow,
	eventRows []events.Envelope,
) error {
	writes := []func() error{
		func() error {
			return parquetwriter.Write(cfg.ParquetPath("bronze", "metadata_records.parquet"), metadataRows)
		},
		func() error { return parquetwriter.Write(cfg.ParquetPath("metadata", "scenes.parquet"), sceneRows) },
		func() error { return parquetwriter.Write(cfg.ParquetPath("metadata", "samples.parquet"), sampleRows) },
		func() error {
			return parquetwriter.Write(cfg.ParquetPath("metadata", "sample_sensors.parquet"), sensorRows)
		},
		func() error {
			return parquetwriter.Write(cfg.ParquetPath("metadata", "sample_data.parquet"), sampleDataRows)
		},
		func() error {
			return parquetwriter.Write(cfg.ParquetPath("metadata", "ego_poses.parquet"), egoPoseRows)
		},
		func() error {
			return parquetwriter.Write(cfg.ParquetPath("metadata", "calibrations.parquet"), calibrationRows)
		},
		func() error {
			return parquetwriter.Write(cfg.ParquetPath("metadata", "annotations.parquet"), annotationRows)
		},
		func() error { return parquetwriter.Write(cfg.ParquetPath("metadata", "maps.parquet"), mapRows) },
		func() error { return parquetwriter.Write(cfg.ParquetPath("features", "can_bus.parquet"), canRows) },
		func() error { return parquetwriter.Write(cfg.ParquetPath("events", "events.parquet"), eventRows) },
	}
	for _, write := range writes {
		if err := write(); err != nil {
			return err
		}
	}
	return nil
}

func filterSceneRows(rows []nuscenes.SceneRow, bundles []nuscenes.SampleBundle) []nuscenes.SceneRow {
	used := map[string]bool{}
	for _, bundle := range bundles {
		used[bundle.SceneID] = true
	}
	filtered := make([]nuscenes.SceneRow, 0, len(used))
	for _, row := range rows {
		if used[row.SceneID] {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

type LiDARSummary struct {
	Files  int
	Points int
}

func WriteLiDARPoints(cfg config.Config, bundles []nuscenes.SampleBundle, assets []rawassets.AssetRow) (LiDARSummary, error) {
	selectedBundles := lidarBundles(cfg, bundles)
	assetsByPath := assetsByCleanPath(assets)
	requiredAssets := map[string]rawassets.AssetRow{}
	for _, bundle := range selectedBundles {
		asset, err := lidarAssetForBundle(bundle, assetsByPath)
		if err != nil {
			return LiDARSummary{}, err
		}
		requiredAssets[asset.AssetID] = asset
	}
	assetBytes, err := rawassets.ReadSelectedAssetBytes(cfg.ParquetPath("bronze", "raw_asset_chunks"), requiredAssets)
	if err != nil {
		return LiDARSummary{}, err
	}

	writer, err := parquetwriter.NewStream[LiDARPointRow](cfg.ParquetPath("features", "lidar_points.parquet"))
	if err != nil {
		return LiDARSummary{}, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = writer.Close()
		}
	}()

	batch := make([]LiDARPointRow, 0, 100000)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := writer.Write(batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	summary := LiDARSummary{}
	for _, bundle := range selectedBundles {
		asset, err := lidarAssetForBundle(bundle, assetsByPath)
		if err != nil {
			return LiDARSummary{}, err
		}
		payload, ok := assetBytes[asset.AssetID]
		if !ok {
			return LiDARSummary{}, fmt.Errorf("sample %s LIDAR_TOP bytes missing from raw asset chunks: %s", bundle.SampleID, asset.AssetID)
		}
		points, err := lidar.ParseBytes(asset.RelativePath, payload, cfg.MaxLiDARPointsPerSample)
		if err != nil {
			return LiDARSummary{}, err
		}
		sourcePointCount := len(payload) / lidar.PointStride
		summary.Files++
		for _, point := range points {
			batch = append(batch, LiDARPointRow{
				SampleID:          bundle.SampleID,
				SceneID:           bundle.SceneID,
				Timestamp:         bundle.Timestamp,
				LiDARAssetID:      asset.AssetID,
				LiDARRelativePath: asset.RelativePath,
				LiDARPath:         asset.Path,
				LiDARSHA256:       asset.SHA256,
				LiDARSizeBytes:    asset.SizeBytes,
				ParserVersion:     lidar.ParserVersion,
				SourcePointCount:  sourcePointCount,
				DecodedPointCount: len(points),
				PointIndex:        point.Index,
				X:                 point.X,
				Y:                 point.Y,
				Z:                 point.Z,
				Intensity:         point.Intensity,
				Ring:              point.Ring,
			})
			summary.Points++
			if len(batch) == cap(batch) {
				if err := flush(); err != nil {
					return LiDARSummary{}, err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return LiDARSummary{}, err
	}
	if summary.Points == 0 {
		return LiDARSummary{}, fmt.Errorf("no LiDAR points were parsed")
	}
	if err := writer.Close(); err != nil {
		return LiDARSummary{}, err
	}
	closed = true
	return summary, nil
}

func lidarAssetForBundle(bundle nuscenes.SampleBundle, assetsByPath map[string]rawassets.AssetRow) (rawassets.AssetRow, error) {
	asset, ok := assetsByPath[filepath.Clean(bundle.LiDAR.Path)]
	if !ok {
		return rawassets.AssetRow{}, fmt.Errorf("sample %s LIDAR_TOP missing from raw asset inventory: %s", bundle.SampleID, bundle.LiDAR.Path)
	}
	return asset, nil
}

func lidarBundles(cfg config.Config, bundles []nuscenes.SampleBundle) []nuscenes.SampleBundle {
	if cfg.MaxLiDARSamples <= 0 || cfg.MaxLiDARSamples >= len(bundles) {
		return bundles
	}
	return bundles[:cfg.MaxLiDARSamples]
}

func WriteJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}
