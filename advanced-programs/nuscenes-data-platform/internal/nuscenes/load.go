package nuscenes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
)

func Load(cfg config.Config) (Dataset, error) {
	var ds Dataset
	loaders := []struct {
		name string
		out  any
	}{
		{"scene", &ds.Scenes},
		{"sample", &ds.Samples},
		{"sample_data", &ds.SampleData},
		{"sensor", &ds.Sensors},
		{"calibrated_sensor", &ds.CalibratedSensors},
		{"ego_pose", &ds.EgoPoses},
		{"sample_annotation", &ds.Annotations},
		{"map", &ds.Maps},
	}
	for _, loader := range loaders {
		if err := readJSON(cfg.MetadataPath(loader.name), loader.out); err != nil {
			return Dataset{}, err
		}
	}
	ds.buildIndexes()
	return ds, nil
}

func readJSON(path string, out any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func (ds *Dataset) buildIndexes() {
	ds.scenesByToken = make(map[string]Scene, len(ds.Scenes))
	for _, scene := range ds.Scenes {
		ds.scenesByToken[scene.Token] = scene
	}
	ds.samplesByToken = make(map[string]Sample, len(ds.Samples))
	for _, sample := range ds.Samples {
		ds.samplesByToken[sample.Token] = sample
	}
	ds.sensorsByToken = make(map[string]Sensor, len(ds.Sensors))
	for _, sensor := range ds.Sensors {
		ds.sensorsByToken[sensor.Token] = sensor
	}
	ds.calibrationsByToken = make(map[string]CalibratedSensor, len(ds.CalibratedSensors))
	for _, calibration := range ds.CalibratedSensors {
		ds.calibrationsByToken[calibration.Token] = calibration
	}
	ds.egoPosesByToken = make(map[string]EgoPose, len(ds.EgoPoses))
	for _, pose := range ds.EgoPoses {
		ds.egoPosesByToken[pose.Token] = pose
	}
	ds.sampleDataBySample = make(map[string][]SampleData)
	for _, sampleData := range ds.SampleData {
		ds.sampleDataBySample[sampleData.SampleToken] = append(ds.sampleDataBySample[sampleData.SampleToken], sampleData)
	}
}

func (ds Dataset) Bundles(cfg config.Config) ([]SampleBundle, error) {
	allowedScenes := ds.allowedSceneTokens(cfg.MaxScenes)
	bundles := make([]SampleBundle, 0, len(ds.Samples))
	for _, sample := range ds.Samples {
		scene, ok := ds.scenesByToken[sample.SceneToken]
		if !ok {
			return nil, fmt.Errorf("sample %s references unknown scene %s", sample.Token, sample.SceneToken)
		}
		if len(allowedScenes) > 0 && !allowedScenes[sample.SceneToken] {
			continue
		}
		bundle, err := ds.bundleForSample(cfg, scene, sample)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, bundle)
		if cfg.MaxSamples > 0 && len(bundles) >= cfg.MaxSamples {
			break
		}
	}
	return bundles, nil
}

func (ds Dataset) allowedSceneTokens(maxScenes int) map[string]bool {
	if maxScenes <= 0 {
		return nil
	}
	allowed := map[string]bool{}
	for i, scene := range ds.Scenes {
		if i >= maxScenes {
			break
		}
		allowed[scene.Token] = true
	}
	return allowed
}

func (ds Dataset) bundleForSample(cfg config.Config, scene Scene, sample Sample) (SampleBundle, error) {
	files := map[string]SensorFile{}
	for _, sampleData := range ds.sampleDataBySample[sample.Token] {
		if !sampleData.IsKeyFrame {
			continue
		}
		file, ok, err := ds.sensorFile(cfg, scene, sampleData)
		if err != nil {
			return SampleBundle{}, err
		}
		if !ok {
			continue
		}
		files[file.SensorChannel] = file
	}
	lidarFile, ok := files["LIDAR_TOP"]
	if !ok {
		return SampleBundle{}, fmt.Errorf("sample %s missing required LIDAR_TOP", sample.Token)
	}
	cameras := map[string]SensorFile{}
	calibrationTokens := []string{lidarFile.CalibratedSensorToken}
	for _, channel := range RequiredCameraChannels {
		file, ok := files[channel]
		if !ok {
			return SampleBundle{}, fmt.Errorf("sample %s missing required camera %s", sample.Token, channel)
		}
		cameras[channel] = file
		calibrationTokens = append(calibrationTokens, file.CalibratedSensorToken)
	}
	sort.Strings(calibrationTokens)
	return SampleBundle{
		SampleID:      sample.Token,
		SceneID:       scene.Name,
		SceneToken:    scene.Token,
		Timestamp:     sample.Timestamp,
		LiDAR:         lidarFile,
		Cameras:       cameras,
		EgoPoseID:     lidarFile.EgoPoseID,
		CalibrationID: shortHash(strings.Join(calibrationTokens, "|")),
	}, nil
}

func (ds Dataset) sensorFile(cfg config.Config, scene Scene, sampleData SampleData) (SensorFile, bool, error) {
	calibration, ok := ds.calibrationsByToken[sampleData.CalibratedSensorToken]
	if !ok {
		return SensorFile{}, false, fmt.Errorf("sample_data %s references unknown calibrated_sensor %s", sampleData.Token, sampleData.CalibratedSensorToken)
	}
	sensor, ok := ds.sensorsByToken[calibration.SensorToken]
	if !ok {
		return SensorFile{}, false, fmt.Errorf("calibrated_sensor %s references unknown sensor %s", calibration.Token, calibration.SensorToken)
	}
	if sensor.Channel != "LIDAR_TOP" && !isRequiredCamera(sensor.Channel) {
		return SensorFile{}, false, nil
	}
	path := cfg.DatasetPath(sampleData.Filename)
	if _, err := os.Stat(path); err != nil {
		return SensorFile{}, false, fmt.Errorf("sample %s %s file missing: %s", sampleData.SampleToken, sensor.Channel, path)
	}
	return SensorFile{
		Token:                 sampleData.Token,
		SampleID:              sampleData.SampleToken,
		SceneID:               scene.Name,
		EgoPoseID:             sampleData.EgoPoseToken,
		CalibratedSensorToken: sampleData.CalibratedSensorToken,
		SensorChannel:         sensor.Channel,
		SensorModality:        sensor.Modality,
		Timestamp:             sampleData.Timestamp,
		FileFormat:            sampleData.FileFormat,
		Width:                 sampleData.Width,
		Height:                sampleData.Height,
		Filename:              sampleData.Filename,
		Path:                  path,
		URI:                   uriFor(cfg, sampleData.Filename, path),
	}, true, nil
}

func isRequiredCamera(channel string) bool {
	for _, required := range RequiredCameraChannels {
		if channel == required {
			return true
		}
	}
	return false
}

func uriFor(cfg config.Config, filename string, path string) string {
	if cfg.PathBaseURI == "" {
		return path
	}
	return strings.TrimRight(cfg.PathBaseURI, "/") + "/" + filepath.ToSlash(filename)
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func jsonString(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}

func VersionHash(value any) string {
	payload, _ := json.Marshal(value)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
