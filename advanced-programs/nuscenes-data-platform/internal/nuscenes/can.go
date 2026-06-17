package nuscenes

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
)

type canRecord map[string]any

func CANRows(cfg config.Config, bundles []SampleBundle) ([]CANBusRow, error) {
	if !cfg.IncludeCANBus {
		return nil, nil
	}
	cache := map[string]sceneCAN{}
	rows := make([]CANBusRow, 0, len(bundles))
	for _, bundle := range bundles {
		streams, ok := cache[bundle.SceneID]
		if !ok {
			var err error
			streams, err = loadSceneCAN(cfg.CanBusDir, bundle.SceneID)
			if err != nil {
				return nil, err
			}
			cache[bundle.SceneID] = streams
		}
		vehicle, vehicleTime := nearest(streams.VehicleInfo, bundle.Timestamp)
		steer, steerTime := nearest(streams.Steering, bundle.Timestamp)
		imu, imuTime := nearest(streams.IMU, bundle.Timestamp)
		rows = append(rows, CANBusRow{
			SceneID:            bundle.SceneID,
			SampleID:           bundle.SampleID,
			Timestamp:          bundle.Timestamp,
			NearestCANTimeUS:   vehicleTime,
			OdomSpeed:          number(vehicle["odom_speed"]),
			SteeringAngle:      firstNumber(steer["value"], vehicle["steer_corrected"]),
			LongitudinalAccel:  number(vehicle["longitudinal_accel"]),
			TransversalAccel:   number(vehicle["transversal_accel"]),
			YawRate:            arrayNumber(imu["rotation_rate"], 2),
			NearestIMUTimeUS:   imuTime,
			NearestSteerTimeUS: steerTime,
		})
	}
	return rows, nil
}

type sceneCAN struct {
	VehicleInfo []canRecord
	Steering    []canRecord
	IMU         []canRecord
}

func loadSceneCAN(root string, sceneID string) (sceneCAN, error) {
	streams := sceneCAN{}
	loaders := []struct {
		suffix string
		out    *[]canRecord
	}{
		{"zoe_veh_info", &streams.VehicleInfo},
		{"steeranglefeedback", &streams.Steering},
		{"ms_imu", &streams.IMU},
	}
	for _, loader := range loaders {
		path := filepath.Join(root, fmt.Sprintf("%s_%s.json", sceneID, loader.suffix))
		payload, err := os.ReadFile(path)
		if err != nil {
			return sceneCAN{}, fmt.Errorf("required CAN stream missing for %s: %s", sceneID, path)
		}
		if err := json.Unmarshal(payload, loader.out); err != nil {
			return sceneCAN{}, fmt.Errorf("parse CAN stream %s: %w", path, err)
		}
	}
	return streams, nil
}

func nearest(records []canRecord, timestamp int64) (canRecord, int64) {
	if len(records) == 0 {
		return canRecord{}, 0
	}
	best := records[0]
	bestTime := int64(number(best["utime"]))
	bestDistance := absInt64(bestTime - timestamp)
	for _, record := range records[1:] {
		recordTime := int64(number(record["utime"]))
		distance := absInt64(recordTime - timestamp)
		if distance < bestDistance {
			best = record
			bestTime = recordTime
			bestDistance = distance
		}
	}
	return best, bestTime
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func number(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func firstNumber(values ...any) float64 {
	for _, value := range values {
		result := number(value)
		if !math.IsNaN(result) && result != 0 {
			return result
		}
	}
	return 0
}

func arrayNumber(value any, index int) float64 {
	items, ok := value.([]any)
	if !ok || index >= len(items) {
		return 0
	}
	return number(items[index])
}
