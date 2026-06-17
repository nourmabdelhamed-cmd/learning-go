package lidar

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

const PointStride = 20
const ParserVersion = "nuscenes-lidar-bin.v1"

type Point struct {
	Index     int64   `parquet:"index"`
	X         float32 `parquet:"x"`
	Y         float32 `parquet:"y"`
	Z         float32 `parquet:"z"`
	Intensity float32 `parquet:"intensity"`
	Ring      float32 `parquet:"ring"`
}

func ParseFile(path string, maxPoints int) ([]Point, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseBytes(path, payload, maxPoints)
}

func ParseBytes(source string, payload []byte, maxPoints int) ([]Point, error) {
	if len(payload) < PointStride {
		return nil, fmt.Errorf("lidar payload %s has %d bytes, need at least %d", source, len(payload), PointStride)
	}
	if len(payload)%PointStride != 0 {
		return nil, fmt.Errorf("lidar payload %s has %d trailing bytes beyond %d-byte point records", source, len(payload)%PointStride, PointStride)
	}
	total := len(payload) / PointStride
	if maxPoints > 0 && total > maxPoints {
		total = maxPoints
	}
	points := make([]Point, 0, total)
	for i := 0; i < total; i++ {
		offset := i * PointStride
		points = append(points, Point{
			Index:     int64(i),
			X:         readFloat32(payload[offset:]),
			Y:         readFloat32(payload[offset+4:]),
			Z:         readFloat32(payload[offset+8:]),
			Intensity: readFloat32(payload[offset+12:]),
			Ring:      readFloat32(payload[offset+16:]),
		})
	}
	return points, nil
}

func readFloat32(payload []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(payload[:4]))
}
