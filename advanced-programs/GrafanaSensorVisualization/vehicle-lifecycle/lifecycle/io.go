package lifecycle

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var telemetryHeader = []string{
	"timestamp",
	"vehicle_id",
	"trip_id",
	"speed_kph",
	"accel_x_mps2",
	"accel_y_mps2",
	"yaw_rate_dps",
	"steering_angle_deg",
	"brake_pressure",
	"wheel_vibration",
	"tire_pressure_psi",
	"battery_voltage",
	"camera_temp_c",
	"lidar_temp_c",
	"gps_accuracy_m",
	"road_friction",
}

var featureHeader = append(append([]string{}, telemetryHeader...),
	"delta_speed_kph",
	"deceleration_mps2",
	"lateral_g",
	"sensor_temp_delta_c",
	"tire_pressure_delta_psi",
	"health_score",
	"sensor_anomaly",
	"harsh_braking",
	"unsafe_maneuver",
	"sensor_drift",
	"vehicle_health_issue",
	"road_condition",
)

func WriteTelemetryCSV(path string, records []TelemetryRecord) error {
	if err := ensureParent(path); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	if err := writer.Write(telemetryHeader); err != nil {
		return err
	}
	for _, record := range records {
		if err := writer.Write(telemetryRow(record)); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func ReadTelemetryCSV(path string) ([]TelemetryRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if _, err := reader.Read(); err == io.EOF {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var records []TelemetryRecord
	line := 1
	for {
		line++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		record, err := parseTelemetryRow(row)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func WriteFeatureCSV(path string, records []FeatureRecord) error {
	if err := ensureParent(path); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	if err := writer.Write(featureHeader); err != nil {
		return err
	}
	for _, record := range records {
		row := append(telemetryRow(record.TelemetryRecord),
			formatFloat(record.DeltaSpeedKPH),
			formatFloat(record.DecelerationMPS2),
			formatFloat(record.LateralG),
			formatFloat(record.SensorTempDeltaC),
			formatFloat(record.TirePressureDelta),
			formatFloat(record.HealthScore),
			strconv.FormatBool(record.SensorAnomaly),
			strconv.FormatBool(record.HarshBraking),
			strconv.FormatBool(record.UnsafeManeuver),
			strconv.FormatBool(record.SensorDrift),
			strconv.FormatBool(record.VehicleHealthIssue),
			record.RoadCondition,
		)
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func ReadFeatureCSV(path string) ([]FeatureRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if _, err := reader.Read(); err == io.EOF {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var features []FeatureRecord
	line := 1
	for {
		line++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		feature, err := parseFeatureRow(row)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		features = append(features, feature)
	}
	return features, nil
}

func WriteJSON(path string, value any) error {
	if err := ensureParent(path); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}

func telemetryRow(record TelemetryRecord) []string {
	return []string{
		record.Timestamp.Format(time.RFC3339Nano),
		record.VehicleID,
		record.TripID,
		formatFloat(record.SpeedKPH),
		formatFloat(record.AccelXMPS2),
		formatFloat(record.AccelYMPS2),
		formatFloat(record.YawRateDPS),
		formatFloat(record.SteeringAngleDeg),
		formatFloat(record.BrakePressure),
		formatFloat(record.WheelVibration),
		formatFloat(record.TirePressurePSI),
		formatFloat(record.BatteryVoltage),
		formatFloat(record.CameraTempC),
		formatFloat(record.LidarTempC),
		formatFloat(record.GPSAccuracyM),
		formatFloat(record.RoadFriction),
	}
}

func parseTelemetryRow(row []string) (TelemetryRecord, error) {
	if len(row) < len(telemetryHeader) {
		return TelemetryRecord{}, fmt.Errorf("expected %d columns, got %d", len(telemetryHeader), len(row))
	}
	timestamp, err := time.Parse(time.RFC3339Nano, row[0])
	if err != nil {
		return TelemetryRecord{}, err
	}
	values, err := parseFloats(row[3:16])
	if err != nil {
		return TelemetryRecord{}, err
	}
	return TelemetryRecord{
		Timestamp:        timestamp,
		VehicleID:        row[1],
		TripID:           row[2],
		SpeedKPH:         values[0],
		AccelXMPS2:       values[1],
		AccelYMPS2:       values[2],
		YawRateDPS:       values[3],
		SteeringAngleDeg: values[4],
		BrakePressure:    values[5],
		WheelVibration:   values[6],
		TirePressurePSI:  values[7],
		BatteryVoltage:   values[8],
		CameraTempC:      values[9],
		LidarTempC:       values[10],
		GPSAccuracyM:     values[11],
		RoadFriction:     values[12],
	}, nil
}

func parseFeatureRow(row []string) (FeatureRecord, error) {
	if len(row) < len(featureHeader) {
		return FeatureRecord{}, fmt.Errorf("expected %d columns, got %d", len(featureHeader), len(row))
	}
	telemetry, err := parseTelemetryRow(row[:len(telemetryHeader)])
	if err != nil {
		return FeatureRecord{}, err
	}
	values, err := parseFloats(row[len(telemetryHeader) : len(telemetryHeader)+6])
	if err != nil {
		return FeatureRecord{}, err
	}
	offset := len(telemetryHeader) + 6
	flags := make([]bool, 5)
	for i := range flags {
		parsed, err := strconv.ParseBool(row[offset+i])
		if err != nil {
			return FeatureRecord{}, err
		}
		flags[i] = parsed
	}
	return FeatureRecord{
		TelemetryRecord:    telemetry,
		DeltaSpeedKPH:      values[0],
		DecelerationMPS2:   values[1],
		LateralG:           values[2],
		SensorTempDeltaC:   values[3],
		TirePressureDelta:  values[4],
		HealthScore:        values[5],
		SensorAnomaly:      flags[0],
		HarshBraking:       flags[1],
		UnsafeManeuver:     flags[2],
		SensorDrift:        flags[3],
		VehicleHealthIssue: flags[4],
		RoadCondition:      row[offset+5],
	}, nil
}

func parseFloats(values []string) ([]float64, error) {
	parsed := make([]float64, len(values))
	for i, value := range values {
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		parsed[i] = floatValue
	}
	return parsed, nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func ensureParent(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0755)
}
