package nuscenes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
)

func (ds Dataset) SceneRows() []SceneRow {
	rows := make([]SceneRow, 0, len(ds.Scenes))
	for _, scene := range ds.Scenes {
		rows = append(rows, SceneRow{
			SceneID:         scene.Name,
			SceneToken:      scene.Token,
			LogToken:        scene.LogToken,
			NumberOfSamples: scene.NumberOfSamples,
			Description:     scene.Description,
		})
	}
	return rows
}

func SampleRows(bundles []SampleBundle) []SampleRow {
	rows := make([]SampleRow, 0, len(bundles))
	for _, bundle := range bundles {
		rows = append(rows, SampleRow{
			SampleID:  bundle.SampleID,
			SceneID:   bundle.SceneID,
			Timestamp: bundle.Timestamp,
		})
	}
	return rows
}

func SampleSensorRows(bundles []SampleBundle) []SampleSensorRow {
	rows := make([]SampleSensorRow, 0, len(bundles)*7)
	for _, bundle := range bundles {
		rows = append(rows, sensorRow(bundle, bundle.LiDAR))
		for _, channel := range RequiredCameraChannels {
			rows = append(rows, sensorRow(bundle, bundle.Cameras[channel]))
		}
	}
	return rows
}

func (ds Dataset) SampleDataRows(cfg config.Config) ([]SampleDataRow, error) {
	rows := make([]SampleDataRow, 0, len(ds.SampleData))
	for _, sampleData := range ds.SampleData {
		sample, ok := ds.samplesByToken[sampleData.SampleToken]
		if !ok {
			return nil, fmt.Errorf("sample_data %s references unknown sample %s", sampleData.Token, sampleData.SampleToken)
		}
		scene, ok := ds.scenesByToken[sample.SceneToken]
		if !ok {
			return nil, fmt.Errorf("sample %s references unknown scene %s", sample.Token, sample.SceneToken)
		}
		calibration, ok := ds.calibrationsByToken[sampleData.CalibratedSensorToken]
		if !ok {
			return nil, fmt.Errorf("sample_data %s references unknown calibrated_sensor %s", sampleData.Token, sampleData.CalibratedSensorToken)
		}
		sensor, ok := ds.sensorsByToken[calibration.SensorToken]
		if !ok {
			return nil, fmt.Errorf("calibrated_sensor %s references unknown sensor %s", calibration.Token, calibration.SensorToken)
		}
		path := cfg.DatasetPath(sampleData.Filename)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("sample_data %s file missing: %s", sampleData.Token, path)
		}
		rows = append(rows, SampleDataRow{
			SampleDataID:          sampleData.Token,
			SampleID:              sampleData.SampleToken,
			SceneID:               scene.Name,
			Timestamp:             sampleData.Timestamp,
			SensorChannel:         sensor.Channel,
			SensorModality:        sensor.Modality,
			FileFormat:            sampleData.FileFormat,
			IsKeyFrame:            sampleData.IsKeyFrame,
			Width:                 sampleData.Width,
			Height:                sampleData.Height,
			Filename:              sampleData.Filename,
			Path:                  path,
			URI:                   uriFor(cfg, sampleData.Filename, path),
			SizeBytes:             info.Size(),
			EgoPoseID:             sampleData.EgoPoseToken,
			CalibratedSensorToken: sampleData.CalibratedSensorToken,
			SensorToken:           calibration.SensorToken,
			Previous:              sampleData.Previous,
			Next:                  sampleData.Next,
		})
	}
	return rows, nil
}

func sensorRow(bundle SampleBundle, file SensorFile) SampleSensorRow {
	return SampleSensorRow{
		SampleID:              bundle.SampleID,
		SceneID:               bundle.SceneID,
		Timestamp:             file.Timestamp,
		SensorChannel:         file.SensorChannel,
		SensorModality:        file.SensorModality,
		FileFormat:            file.FileFormat,
		Width:                 file.Width,
		Height:                file.Height,
		Filename:              file.Filename,
		Path:                  file.Path,
		URI:                   file.URI,
		EgoPoseID:             file.EgoPoseID,
		CalibratedSensorToken: file.CalibratedSensorToken,
		CalibrationID:         bundle.CalibrationID,
	}
}

func (ds Dataset) EgoPoseRows() []EgoPoseRow {
	rows := make([]EgoPoseRow, 0, len(ds.EgoPoses))
	for _, pose := range ds.EgoPoses {
		rows = append(rows, EgoPoseRow{
			EgoPoseID:       pose.Token,
			Timestamp:       pose.Timestamp,
			TranslationJSON: jsonString(pose.Translation),
			RotationJSON:    jsonString(pose.Rotation),
		})
	}
	return rows
}

func (ds Dataset) CalibrationRows() []CalibrationRow {
	rows := make([]CalibrationRow, 0, len(ds.CalibratedSensors))
	for _, calibration := range ds.CalibratedSensors {
		sensor := ds.sensorsByToken[calibration.SensorToken]
		rows = append(rows, CalibrationRow{
			CalibrationID:       VersionHash(calibration),
			CalibratedSensorID:  calibration.Token,
			SensorToken:         calibration.SensorToken,
			SensorChannel:       sensor.Channel,
			SensorModality:      sensor.Modality,
			TranslationJSON:     jsonString(calibration.Translation),
			RotationJSON:        jsonString(calibration.Rotation),
			CameraIntrinsicJSON: jsonString(calibration.CameraIntrinsic),
		})
	}
	return rows
}

func (ds Dataset) AnnotationRows() []AnnotationRow {
	rows := make([]AnnotationRow, 0, len(ds.Annotations))
	for _, annotation := range ds.Annotations {
		rows = append(rows, AnnotationRow{
			AnnotationID:   annotation.Token,
			SampleID:       annotation.SampleToken,
			InstanceToken:  annotation.InstanceToken,
			NumLiDARPoints: annotation.NumLiDARPoints,
			NumRadarPoints: annotation.NumRadarPoints,
			SizeJSON:       jsonString(annotation.Size),
		})
	}
	return rows
}

func (ds Dataset) MapRows(cfg config.Config) []MapRow {
	rows := make([]MapRow, 0, len(ds.Maps))
	for _, item := range ds.Maps {
		path := cfg.DatasetPath(item.Filename)
		rows = append(rows, MapRow{
			MapID:     item.Token,
			Category:  item.Category,
			Filename:  item.Filename,
			Path:      path,
			Source:    "nuscenes-mini",
			LogTokens: strings.Join(item.LogTokens, ","),
		})
	}
	rows = append(rows, expansionMapRows(cfg.MapExpansionDir)...)
	return rows
}

func expansionMapRows(root string) []MapRow {
	if root == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(root, "expansion"))
	if err != nil {
		return nil
	}
	rows := make([]MapRow, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(root, "expansion", entry.Name())
		description := ""
		payload, err := os.ReadFile(path)
		if err == nil {
			var decoded map[string]any
			if json.Unmarshal(payload, &decoded) == nil {
				if value, ok := decoded["version"].(string); ok {
					description = value
				}
			}
		}
		rows = append(rows, MapRow{
			MapID:       strings.TrimSuffix(entry.Name(), ".json"),
			Category:    "map_expansion",
			Filename:    filepath.ToSlash(filepath.Join("expansion", entry.Name())),
			Path:        path,
			Source:      "nuScenes-map-expansion-v1.3",
			Description: description,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].MapID < rows[j].MapID })
	return rows
}
