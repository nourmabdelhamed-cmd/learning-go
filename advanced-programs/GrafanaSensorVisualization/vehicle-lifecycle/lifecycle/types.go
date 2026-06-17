package lifecycle

import "time"

type Config struct {
	Rows       int
	Vehicles   int
	Seed       int64
	DataDir    string
	Thresholds Thresholds
}

type Thresholds struct {
	HarshBrakeAccel       float64
	HarshBrakePressure    float64
	UnsafeLateralAccel    float64
	UnsafeMinSpeedKPH     float64
	UnsafeYawRateDPS      float64
	UnsafeSteeringDeg     float64
	SensorAnomalyGPSM     float64
	SensorDriftTempDeltaC float64
	LowTirePressurePSI    float64
	LowBatteryVoltage     float64
	HighWheelVibration    float64
	IcyRoadFriction       float64
	WetRoadFriction       float64
	RoughRoadVibration    float64
}

func DefaultConfig() Config {
	return Config{
		Rows:     1500,
		Vehicles: 4,
		Seed:     42,
		DataDir:  "data",
		Thresholds: Thresholds{
			HarshBrakeAccel:       -5.5,
			HarshBrakePressure:    0.65,
			UnsafeLateralAccel:    4.2,
			UnsafeMinSpeedKPH:     45,
			UnsafeYawRateDPS:      35,
			UnsafeSteeringDeg:     18,
			SensorAnomalyGPSM:     20,
			SensorDriftTempDeltaC: 8,
			LowTirePressurePSI:    30,
			LowBatteryVoltage:     11.8,
			HighWheelVibration:    0.75,
			IcyRoadFriction:       0.42,
			WetRoadFriction:       0.68,
			RoughRoadVibration:    0.68,
		},
	}
}

type TelemetryRecord struct {
	Timestamp        time.Time
	VehicleID        string
	TripID           string
	SpeedKPH         float64
	AccelXMPS2       float64
	AccelYMPS2       float64
	YawRateDPS       float64
	SteeringAngleDeg float64
	BrakePressure    float64
	WheelVibration   float64
	TirePressurePSI  float64
	BatteryVoltage   float64
	CameraTempC      float64
	LidarTempC       float64
	GPSAccuracyM     float64
	RoadFriction     float64
}

type FeatureRecord struct {
	TelemetryRecord
	DeltaSpeedKPH      float64
	DecelerationMPS2   float64
	LateralG           float64
	SensorTempDeltaC   float64
	TirePressureDelta  float64
	HealthScore        float64
	SensorAnomaly      bool
	HarshBraking       bool
	UnsafeManeuver     bool
	SensorDrift        bool
	VehicleHealthIssue bool
	RoadCondition      string
}

type QualityReport struct {
	TotalRecords   int            `json:"total_records"`
	ValidRecords   int            `json:"valid_records"`
	InvalidRecords int            `json:"invalid_records"`
	IssueCounts    map[string]int `json:"issue_counts"`
}

type MonitoringSummary struct {
	TotalRecords        int            `json:"total_records"`
	SensorAnomalies     int            `json:"sensor_anomalies"`
	HarshBrakingEvents  int            `json:"harsh_braking_events"`
	UnsafeManeuvers     int            `json:"unsafe_maneuvers"`
	SensorDriftEvents   int            `json:"sensor_drift_events"`
	VehicleHealthIssues int            `json:"vehicle_health_issues"`
	RoadConditions      map[string]int `json:"road_conditions"`
}
