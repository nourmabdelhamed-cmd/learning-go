package lifecycle

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

func GenerateTelemetry(rows int, vehicles int, seed int64) []TelemetryRecord {
	if rows < 1 {
		rows = 1
	}
	if vehicles < 1 {
		vehicles = 1
	}

	rng := rand.New(rand.NewSource(seed))
	start := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)
	records := make([]TelemetryRecord, 0, rows)

	for i := 0; i < rows; i++ {
		vehicleNumber := i%vehicles + 1
		vehicleID := fmt.Sprintf("av-%03d", vehicleNumber)
		tripID := fmt.Sprintf("trip-%03d", i/(vehicles*220)+1)

		speed := clamp(55+24*math.Sin(float64(i)/73)+rng.NormFloat64()*4.5, 0, 125)
		accelX := rng.NormFloat64() * 0.7
		accelY := rng.NormFloat64() * 0.45
		yawRate := rng.NormFloat64() * 6
		steering := rng.NormFloat64() * 5
		brake := clamp(0.05+rng.Float64()*0.14, 0, 1)
		vibration := clamp(0.12+rng.Float64()*0.16, 0, 1)
		tire := 32 + rng.NormFloat64()*0.8
		battery := 12.6 + rng.NormFloat64()*0.18
		cameraTemp := 36 + 5*math.Sin(float64(i)/110) + rng.NormFloat64()*0.7
		lidarTemp := 35.5 + 4.5*math.Sin(float64(i)/118) + rng.NormFloat64()*0.7
		gpsAccuracy := 1.2 + rng.Float64()*1.8
		roadFriction := 0.86 + rng.NormFloat64()*0.03

		if i > 10 && i%137 < 6 {
			accelX = -6.5 - rng.Float64()*2.2
			brake = 0.78 + rng.Float64()*0.18
			speed = clamp(speed+15, 50, 130)
		}

		if i%211 >= 30 && i%211 < 37 {
			accelY = signByIndex(i) * (4.7 + rng.Float64()*1.4)
			yawRate = signByIndex(i) * (38 + rng.Float64()*16)
			steering = signByIndex(i) * (20 + rng.Float64()*14)
			speed = clamp(speed+10, 48, 125)
		}

		if i%149 == 17 {
			gpsAccuracy = 24 + rng.Float64()*14
		}

		if vehicleNumber == vehicles && i > rows/3 {
			drift := math.Min(15, 4+float64(i-rows/3)/55)
			cameraTemp += drift
		}

		if i%181 >= 70 && i%181 < 79 {
			tire = 27.5 + rng.Float64()*1.4
			battery = 11.2 + rng.Float64()*0.35
			vibration = 0.78 + rng.Float64()*0.14
		}

		if i%521 >= 100 && i%521 < 116 {
			roadFriction = 0.28 + rng.Float64()*0.11
			vibration = math.Max(vibration, 0.48+rng.Float64()*0.22)
		} else if i%307 > 240 {
			roadFriction = 0.52 + rng.Float64()*0.12
		}

		if i%193 >= 80 && i%193 < 96 {
			vibration = 0.7 + rng.Float64()*0.22
		}

		records = append(records, TelemetryRecord{
			Timestamp:        start.Add(time.Duration(i) * time.Second),
			VehicleID:        vehicleID,
			TripID:           tripID,
			SpeedKPH:         speed,
			AccelXMPS2:       accelX,
			AccelYMPS2:       accelY,
			YawRateDPS:       yawRate,
			SteeringAngleDeg: steering,
			BrakePressure:    brake,
			WheelVibration:   clamp(vibration, 0, 1),
			TirePressurePSI:  tire,
			BatteryVoltage:   battery,
			CameraTempC:      cameraTemp,
			LidarTempC:       lidarTemp,
			GPSAccuracyM:     gpsAccuracy,
			RoadFriction:     clamp(roadFriction, 0, 1),
		})
	}

	return records
}

func signByIndex(i int) float64 {
	if i%2 == 0 {
		return 1
	}
	return -1
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
