package workflow

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/nuscenes"
)

func InspectSample(cfg config.Config, sampleID string) (string, error) {
	if err := cfg.RequireRawData(); err != nil {
		return "", err
	}
	ds, err := nuscenes.Load(cfg)
	if err != nil {
		return "", err
	}
	bundles, err := ds.Bundles(cfg)
	if err != nil {
		return "", err
	}
	if len(bundles) == 0 {
		return "", fmt.Errorf("no valid samples found")
	}
	selected := bundles[0]
	if sampleID != "" {
		found := false
		for _, bundle := range bundles {
			if bundle.SampleID == sampleID {
				selected = bundle
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("sample %s not found in valid sample set", sampleID)
		}
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "sample_id=%s\n", selected.SampleID)
	fmt.Fprintf(&out, "scene_id=%s\n", selected.SceneID)
	fmt.Fprintf(&out, "timestamp=%d\n", selected.Timestamp)
	fmt.Fprintf(&out, "LIDAR_TOP=%s\n", selected.LiDAR.Path)
	channels := make([]string, 0, len(selected.Cameras))
	for channel := range selected.Cameras {
		channels = append(channels, channel)
	}
	sort.Strings(channels)
	for _, channel := range channels {
		fmt.Fprintf(&out, "%s=%s\n", channel, selected.Cameras[channel].Path)
	}
	return out.String(), nil
}
