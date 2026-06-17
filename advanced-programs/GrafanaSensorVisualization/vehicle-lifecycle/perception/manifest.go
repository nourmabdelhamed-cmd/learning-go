package perception

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ManifestRecord struct {
	Dataset string `json:"dataset"`
	FrameID string `json:"frame_id"`
	Split   string `json:"split"`
	Image   string `json:"image"`
	Mask    string `json:"mask"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type ManifestSummary struct {
	Dataset      string         `json:"dataset"`
	Archive      string         `json:"archive"`
	Manifest     string         `json:"manifest"`
	TotalPairs   int            `json:"total_pairs"`
	ImageFiles   int            `json:"image_files"`
	MaskFiles    int            `json:"mask_files"`
	OrphanImages int            `json:"orphan_images"`
	OrphanMasks  int            `json:"orphan_masks"`
	SplitCounts  map[string]int `json:"split_counts"`
	InvalidPairs []string       `json:"invalid_pairs,omitempty"`
}

func BuildCamVidManifest(zipPath string, manifestPath string, summaryPath string) (ManifestSummary, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return ManifestSummary{}, err
	}
	defer reader.Close()

	images := map[string]*zip.File{}
	masks := map[string]*zip.File{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name))
		key := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
		switch ext {
		case ".jpg", ".jpeg":
			images[key] = file
		case ".png":
			masks[key] = file
		}
	}

	keys := make([]string, 0, len(images))
	for key := range images {
		if _, ok := masks[key]; ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	records := make([]ManifestRecord, 0, len(keys))
	invalidPairs := []string{}
	for index, key := range keys {
		imageFile := images[key]
		maskFile := masks[key]
		width, height, err := imageDimensions(imageFile)
		if err != nil {
			invalidPairs = append(invalidPairs, fmt.Sprintf("%s image: %v", key, err))
			continue
		}
		maskWidth, maskHeight, err := imageDimensions(maskFile)
		if err != nil {
			invalidPairs = append(invalidPairs, fmt.Sprintf("%s mask: %v", key, err))
			continue
		}
		if width != maskWidth || height != maskHeight {
			invalidPairs = append(invalidPairs, fmt.Sprintf("%s dimensions: image=%dx%d mask=%dx%d", key, width, height, maskWidth, maskHeight))
			continue
		}

		records = append(records, ManifestRecord{
			Dataset: "camvid-bluechannel",
			FrameID: filepath.Base(key),
			Split:   splitForIndex(index, len(keys)),
			Image:   zipURI(zipPath, imageFile.Name),
			Mask:    zipURI(zipPath, maskFile.Name),
			Width:   width,
			Height:  height,
		})
	}

	if err := writeJSONL(manifestPath, records); err != nil {
		return ManifestSummary{}, err
	}

	summary := ManifestSummary{
		Dataset:      "camvid-bluechannel",
		Archive:      zipPath,
		Manifest:     manifestPath,
		TotalPairs:   len(records),
		ImageFiles:   len(images),
		MaskFiles:    len(masks),
		OrphanImages: countOrphans(images, masks),
		OrphanMasks:  countOrphans(masks, images),
		SplitCounts:  splitCounts(records),
		InvalidPairs: invalidPairs,
	}
	if summaryPath != "" {
		if err := writeJSON(summaryPath, summary); err != nil {
			return ManifestSummary{}, err
		}
	}
	return summary, nil
}

func imageDimensions(file *zip.File) (int, int, error) {
	reader, err := file.Open()
	if err != nil {
		return 0, 0, err
	}
	defer reader.Close()

	config, _, err := image.DecodeConfig(reader)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}

func splitForIndex(index int, total int) string {
	trainLimit := total * 8 / 10
	valLimit := total * 9 / 10
	switch {
	case index < trainLimit:
		return "train"
	case index < valLimit:
		return "val"
	default:
		return "test"
	}
}

func zipURI(zipPath string, entry string) string {
	return "zip://" + filepath.ToSlash(zipPath) + "!/" + entry
}

func countOrphans(left map[string]*zip.File, right map[string]*zip.File) int {
	count := 0
	for key := range left {
		if _, ok := right[key]; !ok {
			count++
		}
	}
	return count
}

func splitCounts(records []ManifestRecord) map[string]int {
	counts := map[string]int{"train": 0, "val": 0, "test": 0}
	for _, record := range records {
		counts[record.Split]++
	}
	return counts
}

func writeJSONL(path string, records []ManifestRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}
