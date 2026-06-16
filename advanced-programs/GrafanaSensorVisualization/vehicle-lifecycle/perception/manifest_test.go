package perception

import (
	"archive/zip"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCamVidManifestPairsImagesAndMasks(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "camvid.zip")
	if err := writeTestZip(zipPath); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(tmp, "manifest.jsonl")
	summaryPath := filepath.Join(tmp, "summary.json")

	summary, err := BuildCamVidManifest(zipPath, manifestPath, summaryPath)
	if err != nil {
		t.Fatalf("BuildCamVidManifest returned error: %v", err)
	}
	if summary.TotalPairs != 3 {
		t.Fatalf("total pairs = %d, want 3", summary.TotalPairs)
	}
	if summary.OrphanImages != 1 || summary.OrphanMasks != 1 {
		t.Fatalf("orphan counts = images %d masks %d", summary.OrphanImages, summary.OrphanMasks)
	}
	if summary.SplitCounts["train"] != 2 || summary.SplitCounts["val"] != 0 || summary.SplitCounts["test"] != 1 {
		t.Fatalf("split counts = %#v", summary.SplitCounts)
	}

	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if strings.Count(text, "\n") != 3 {
		t.Fatalf("manifest rows = %d, want 3", strings.Count(text, "\n"))
	}
	if !strings.Contains(text, "zip://") || !strings.Contains(text, `"width":4`) {
		t.Fatalf("manifest payload missing expected fields: %s", text)
	}
	if _, err := os.Stat(summaryPath); err != nil {
		t.Fatalf("summary was not written: %v", err)
	}
}

func writeTestZip(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	archive := zip.NewWriter(file)
	defer archive.Close()

	for _, name := range []string{"frame_001", "frame_002", "frame_003"} {
		if err := addJPEG(archive, "bluechannel/"+name+".jpg"); err != nil {
			return err
		}
		if err := addPNG(archive, "bluechannel/"+name+".png"); err != nil {
			return err
		}
	}
	if err := addJPEG(archive, "bluechannel/orphan_image.jpg"); err != nil {
		return err
	}
	return addPNG(archive, "bluechannel/orphan_mask.png")
}

func addJPEG(archive *zip.Writer, name string) error {
	writer, err := archive.Create(name)
	if err != nil {
		return err
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	return jpeg.Encode(writer, img, nil)
}

func addPNG(archive *zip.Writer, name string) error {
	writer, err := archive.Create(name)
	if err != nil {
		return err
	}
	img := image.NewPaletted(image.Rect(0, 0, 4, 3), []color.Color{color.Black, color.White})
	return png.Encode(writer, img)
}
