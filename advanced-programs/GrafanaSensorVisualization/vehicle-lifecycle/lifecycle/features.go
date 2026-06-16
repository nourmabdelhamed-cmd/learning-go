package lifecycle

import "math"

func BuildFeatures(records []TelemetryRecord, thresholds Thresholds) []FeatureRecord {
	features := make([]FeatureRecord, 0, len(records))
	previousByVehicle := map[string]TelemetryRecord{}

	for _, record := range records {
		previous, hasPrevious := previousByVehicle[record.VehicleID]
		deltaSpeed := 0.0
		if hasPrevious {
			deltaSpeed = record.SpeedKPH - previous.SpeedKPH
		}

		tempDelta := math.Abs(record.CameraTempC - record.LidarTempC)
		tirePressureDelta := 32 - record.TirePressurePSI
		harshBraking := record.AccelXMPS2 <= thresholds.HarshBrakeAccel &&
			record.BrakePressure >= thresholds.HarshBrakePressure
		unsafeManeuver := harshBraking ||
			(math.Abs(record.AccelYMPS2) >= thresholds.UnsafeLateralAccel && record.SpeedKPH >= thresholds.UnsafeMinSpeedKPH) ||
			(math.Abs(record.YawRateDPS) >= thresholds.UnsafeYawRateDPS &&
				math.Abs(record.SteeringAngleDeg) >= thresholds.UnsafeSteeringDeg &&
				record.SpeedKPH >= thresholds.UnsafeMinSpeedKPH)
		sensorAnomaly := record.GPSAccuracyM >= thresholds.SensorAnomalyGPSM ||
			math.Abs(record.AccelXMPS2) > 11 ||
			math.Abs(record.AccelYMPS2) > 11
		sensorDrift := tempDelta >= thresholds.SensorDriftTempDeltaC
		vehicleHealthIssue := record.TirePressurePSI <= thresholds.LowTirePressurePSI ||
			record.BatteryVoltage <= thresholds.LowBatteryVoltage ||
			record.WheelVibration >= thresholds.HighWheelVibration

		features = append(features, FeatureRecord{
			TelemetryRecord:    record,
			DeltaSpeedKPH:      deltaSpeed,
			DecelerationMPS2:   math.Max(0, -record.AccelXMPS2),
			LateralG:           math.Abs(record.AccelYMPS2) / 9.80665,
			SensorTempDeltaC:   tempDelta,
			TirePressureDelta:  tirePressureDelta,
			HealthScore:        healthScore(record, thresholds),
			SensorAnomaly:      sensorAnomaly,
			HarshBraking:       harshBraking,
			UnsafeManeuver:     unsafeManeuver,
			SensorDrift:        sensorDrift,
			VehicleHealthIssue: vehicleHealthIssue,
			RoadCondition:      classifyRoad(record, thresholds),
		})

		previousByVehicle[record.VehicleID] = record
	}

	return features
}

func SummarizeFeatures(features []FeatureRecord) MonitoringSummary {
	summary := MonitoringSummary{
		TotalRecords:   len(features),
		RoadConditions: map[string]int{},
	}

	for _, feature := range features {
		if feature.SensorAnomaly {
			summary.SensorAnomalies++
		}
		if feature.HarshBraking {
			summary.HarshBrakingEvents++
		}
		if feature.UnsafeManeuver {
			summary.UnsafeManeuvers++
		}
		if feature.SensorDrift {
			summary.SensorDriftEvents++
		}
		if feature.VehicleHealthIssue {
			summary.VehicleHealthIssues++
		}
		summary.RoadConditions[feature.RoadCondition]++
	}

	return summary
}

func EventRecords(features []FeatureRecord) []FeatureRecord {
	events := make([]FeatureRecord, 0)
	for _, feature := range features {
		if feature.SensorAnomaly ||
			feature.HarshBraking ||
			feature.UnsafeManeuver ||
			feature.SensorDrift ||
			feature.VehicleHealthIssue ||
			feature.RoadCondition != "dry" {
			events = append(events, feature)
		}
	}
	return events
}

func classifyRoad(record TelemetryRecord, thresholds Thresholds) string {
	if record.RoadFriction <= thresholds.IcyRoadFriction {
		return "icy"
	}
	if record.WheelVibration >= thresholds.RoughRoadVibration {
		return "rough"
	}
	if record.RoadFriction <= thresholds.WetRoadFriction {
		return "wet"
	}
	return "dry"
}

func healthScore(record TelemetryRecord, thresholds Thresholds) float64 {
	score := 100.0
	if record.TirePressurePSI < thresholds.LowTirePressurePSI {
		score -= (thresholds.LowTirePressurePSI - record.TirePressurePSI) * 8
	}
	if record.BatteryVoltage < thresholds.LowBatteryVoltage {
		score -= (thresholds.LowBatteryVoltage - record.BatteryVoltage) * 18
	}
	if record.WheelVibration > thresholds.HighWheelVibration {
		score -= (record.WheelVibration - thresholds.HighWheelVibration) * 40
	}
	return clamp(score, 0, 100)
}
