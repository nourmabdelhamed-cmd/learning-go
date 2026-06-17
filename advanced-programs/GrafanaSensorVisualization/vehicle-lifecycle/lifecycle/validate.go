package lifecycle

import "time"

type ValidationResult struct {
	ValidRecords []TelemetryRecord
	Report       QualityReport
}

func ValidateTelemetry(records []TelemetryRecord) ValidationResult {
	report := QualityReport{
		TotalRecords: len(records),
		IssueCounts:  map[string]int{},
	}
	valid := make([]TelemetryRecord, 0, len(records))

	for _, record := range records {
		issues := validateRecord(record)
		if len(issues) == 0 {
			valid = append(valid, record)
			continue
		}
		for _, issue := range issues {
			report.IssueCounts[issue]++
		}
	}

	report.ValidRecords = len(valid)
	report.InvalidRecords = report.TotalRecords - report.ValidRecords

	return ValidationResult{ValidRecords: valid, Report: report}
}

func validateRecord(record TelemetryRecord) []string {
	var issues []string

	if record.Timestamp.IsZero() || record.Timestamp.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		issues = append(issues, "invalid_timestamp")
	}
	if record.VehicleID == "" {
		issues = append(issues, "missing_vehicle_id")
	}
	if record.TripID == "" {
		issues = append(issues, "missing_trip_id")
	}
	if record.SpeedKPH < 0 || record.SpeedKPH > 180 {
		issues = append(issues, "speed_out_of_range")
	}
	if record.AccelXMPS2 < -15 || record.AccelXMPS2 > 15 {
		issues = append(issues, "accel_x_out_of_range")
	}
	if record.AccelYMPS2 < -15 || record.AccelYMPS2 > 15 {
		issues = append(issues, "accel_y_out_of_range")
	}
	if record.BrakePressure < 0 || record.BrakePressure > 1 {
		issues = append(issues, "brake_pressure_out_of_range")
	}
	if record.WheelVibration < 0 || record.WheelVibration > 1.5 {
		issues = append(issues, "wheel_vibration_out_of_range")
	}
	if record.TirePressurePSI < 20 || record.TirePressurePSI > 45 {
		issues = append(issues, "tire_pressure_out_of_range")
	}
	if record.BatteryVoltage < 9 || record.BatteryVoltage > 15 {
		issues = append(issues, "battery_voltage_out_of_range")
	}
	if record.CameraTempC < -40 || record.CameraTempC > 100 {
		issues = append(issues, "camera_temp_out_of_range")
	}
	if record.LidarTempC < -40 || record.LidarTempC > 100 {
		issues = append(issues, "lidar_temp_out_of_range")
	}
	if record.GPSAccuracyM < 0 || record.GPSAccuracyM > 100 {
		issues = append(issues, "gps_accuracy_out_of_range")
	}
	if record.RoadFriction < 0 || record.RoadFriction > 1 {
		issues = append(issues, "road_friction_out_of_range")
	}

	return issues
}
