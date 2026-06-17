package rawassets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/parquetwriter"
)

func TestWriteRawAssetsCanReconstructFixtureFile(t *testing.T) {
	root := filepath.Join("..", "..", "tests", "fixtures", "nuscenes-mini-tiny")
	runRoot := t.TempDir()
	cfg := config.Config{
		DatasetVersion:     "fixture",
		SchemaVersion:      "manifest.v1",
		RawRoot:            root,
		RawAssetRoots:      []string{root},
		LakeDir:            filepath.Join(runRoot, "lake"),
		EventLogPath:       filepath.Join(runRoot, "events", "events.jsonl"),
		ManifestPath:       filepath.Join(runRoot, "manifest.jsonl"),
		IncludeRawAssets:   true,
		RawAssetChunkBytes: 7,
	}

	assets, summary, err := Write(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Assets == 0 || summary.Chunks == 0 || summary.Bytes == 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	rel := "samples/LIDAR_TOP/sample-token-1__LIDAR_TOP.bin"
	var asset AssetRow
	for _, row := range assets {
		if row.RelativePath == rel {
			asset = row
			break
		}
	}
	if asset.AssetID == "" {
		t.Fatalf("asset %s not found", rel)
	}

	chunkFiles, err := filepath.Glob(filepath.Join(cfg.ParquetPath("bronze", "raw_asset_chunks"), "*.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunkFiles) == 0 {
		t.Fatalf("expected raw asset chunk parquet files")
	}
	chunks := []ChunkRow{}
	for _, path := range chunkFiles {
		rows, err := parquetwriter.Read[ChunkRow](path)
		if err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, rows...)
	}
	filtered := []ChunkRow{}
	for _, chunk := range chunks {
		if chunk.AssetID == asset.AssetID {
			filtered = append(filtered, chunk)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ChunkIndex < filtered[j].ChunkIndex })
	reconstructed := []byte{}
	for _, chunk := range filtered {
		reconstructed = append(reconstructed, chunk.Bytes...)
	}
	original, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reconstructed, original) {
		t.Fatalf("reconstructed bytes do not match original")
	}
	sum := sha256.Sum256(original)
	if asset.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha mismatch: got %s", asset.SHA256)
	}
}
