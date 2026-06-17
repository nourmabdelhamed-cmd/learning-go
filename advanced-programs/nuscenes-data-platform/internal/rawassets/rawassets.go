package rawassets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/parquet-go/parquet-go"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/parquetwriter"
)

type AssetRow struct {
	AssetID        string `parquet:"asset_id"`
	DatasetVersion string `parquet:"dataset_version"`
	SchemaVersion  string `parquet:"schema_version"`
	SourceRoot     string `parquet:"source_root"`
	DatasetArea    string `parquet:"dataset_area"`
	RelativePath   string `parquet:"relative_path"`
	Path           string `parquet:"path"`
	URI            string `parquet:"uri"`
	Extension      string `parquet:"extension"`
	MediaType      string `parquet:"media_type"`
	SizeBytes      int64  `parquet:"size_bytes"`
	SHA256         string `parquet:"sha256"`
	ChunkCount     int    `parquet:"chunk_count"`
	ModifiedUnixNS int64  `parquet:"modified_unix_ns"`
}

type ChunkRow struct {
	AssetID      string `parquet:"asset_id"`
	RelativePath string `parquet:"relative_path"`
	ChunkIndex   int    `parquet:"chunk_index"`
	OffsetBytes  int64  `parquet:"offset_bytes"`
	SizeBytes    int    `parquet:"size_bytes"`
	SHA256       string `parquet:"sha256"`
	Bytes        []byte `parquet:"bytes"`
}

type Summary struct {
	Assets     int   `json:"assets"`
	Chunks     int   `json:"chunks"`
	Bytes      int64 `json:"bytes"`
	ChunkBytes int   `json:"chunk_bytes"`
}

type candidate struct {
	root string
	path string
	rel  string
	info os.FileInfo
}

func Write(cfg config.Config) ([]AssetRow, Summary, error) {
	if !cfg.IncludeRawAssets {
		return nil, Summary{}, nil
	}
	chunks, err := newChunkSink(cfg.ParquetPath("bronze", "raw_asset_chunks"))
	if err != nil {
		return nil, Summary{}, err
	}

	files, err := discover(cfg.RawAssetRoots)
	if err != nil {
		return nil, Summary{}, err
	}

	rows := make([]AssetRow, 0, len(files))
	summary := Summary{ChunkBytes: cfg.RawAssetChunkBytes}
	for _, file := range files {
		row, chunkCount, err := writeFileChunks(cfg, chunks, file)
		if err != nil {
			return nil, Summary{}, err
		}
		rows = append(rows, row)
		summary.Assets++
		summary.Chunks += chunkCount
		summary.Bytes += row.SizeBytes
	}
	if err := chunks.Close(); err != nil {
		return nil, Summary{}, err
	}
	if err := parquetwriter.Write(cfg.ParquetPath("bronze", "raw_assets.parquet"), rows); err != nil {
		return nil, Summary{}, err
	}
	return rows, summary, nil
}

func ReadSelectedAssetBytes(chunkDir string, assets map[string]AssetRow) (map[string][]byte, error) {
	payloads := map[string][]byte{}
	if len(assets) == 0 {
		return payloads, nil
	}

	files, err := filepath.Glob(filepath.Join(chunkDir, "*.parquet"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("raw asset chunks missing: %s/*.parquet", chunkDir)
	}
	sort.Strings(files)

	chunksByAsset := map[string][]ChunkRow{}
	for _, path := range files {
		chunks, err := parquetwriter.Read[ChunkRow](path)
		if err != nil {
			return nil, err
		}
		for _, chunk := range chunks {
			if _, ok := assets[chunk.AssetID]; ok {
				chunksByAsset[chunk.AssetID] = append(chunksByAsset[chunk.AssetID], chunk)
			}
		}
	}

	for assetID, asset := range assets {
		chunks := chunksByAsset[assetID]
		if len(chunks) == 0 && asset.SizeBytes > 0 {
			return nil, fmt.Errorf("raw asset %s has no chunks", assetID)
		}
		if len(chunks) != asset.ChunkCount {
			return nil, fmt.Errorf("raw asset %s chunk count mismatch: got %d, want %d", assetID, len(chunks), asset.ChunkCount)
		}
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].ChunkIndex < chunks[j].ChunkIndex
		})
		payload := make([]byte, 0, int(asset.SizeBytes))
		for index, chunk := range chunks {
			if chunk.ChunkIndex != index {
				return nil, fmt.Errorf("raw asset %s chunk index mismatch: got %d, want %d", assetID, chunk.ChunkIndex, index)
			}
			if chunk.SizeBytes != len(chunk.Bytes) {
				return nil, fmt.Errorf("raw asset %s chunk %d size mismatch: got %d bytes, want %d", assetID, chunk.ChunkIndex, len(chunk.Bytes), chunk.SizeBytes)
			}
			if digestBytes(chunk.Bytes) != chunk.SHA256 {
				return nil, fmt.Errorf("raw asset %s chunk %d sha mismatch", assetID, chunk.ChunkIndex)
			}
			payload = append(payload, chunk.Bytes...)
		}
		if err := VerifyAssetBytes(asset, payload); err != nil {
			return nil, err
		}
		payloads[assetID] = payload
	}
	return payloads, nil
}

func VerifyAssetBytes(asset AssetRow, payload []byte) error {
	if int64(len(payload)) != asset.SizeBytes {
		return fmt.Errorf("raw asset %s size mismatch: got %d bytes, want %d", asset.AssetID, len(payload), asset.SizeBytes)
	}
	if digestBytes(payload) != asset.SHA256 {
		return fmt.Errorf("raw asset %s sha mismatch", asset.AssetID)
	}
	return nil
}

func discover(roots []string) ([]candidate, error) {
	seen := map[string]bool{}
	files := []candidate{}
	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if err := filepath.WalkDir(cleanRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			absKey, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if seen[absKey] {
				return nil
			}
			seen[absKey] = true
			info, err := entry.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(cleanRoot, path)
			if err != nil {
				return err
			}
			files = append(files, candidate{
				root: cleanRoot,
				path: path,
				rel:  filepath.ToSlash(rel),
				info: info,
			})
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].root == files[j].root {
			return files[i].rel < files[j].rel
		}
		return files[i].root < files[j].root
	})
	return files, nil
}

type chunkSink struct {
	dir       string
	partIndex int
	batch     []ChunkRow
}

func newChunkSink(dir string) (*chunkSink, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &chunkSink{
		dir:   dir,
		batch: make([]ChunkRow, 0, 8),
	}, nil
}

func (s *chunkSink) Write(row ChunkRow) error {
	s.batch = append(s.batch, row)
	if len(s.batch) < cap(s.batch) {
		return nil
	}
	return s.flush()
}

func (s *chunkSink) Close() error {
	return s.flush()
}

func (s *chunkSink) flush() error {
	if len(s.batch) == 0 {
		return nil
	}
	path := filepath.Join(s.dir, fmt.Sprintf("part-%06d.parquet", s.partIndex))
	if err := parquetwriter.Write(
		path,
		s.batch,
		parquet.DefaultEncoding(&parquet.Plain),
		parquet.MaxRowsPerRowGroup(int64(len(s.batch))),
		parquet.DataPageStatistics(false),
	); err != nil {
		return err
	}
	for i := range s.batch {
		s.batch[i].Bytes = nil
	}
	s.batch = s.batch[:0]
	s.partIndex++
	runtime.GC()
	return nil
}

func writeFileChunks(cfg config.Config, writer *chunkSink, file candidate) (AssetRow, int, error) {
	input, err := os.Open(file.path)
	if err != nil {
		return AssetRow{}, 0, err
	}
	defer input.Close()

	assetID := assetID(cfg, file.root, file.rel)
	totalHash := sha256.New()
	buffer := make([]byte, cfg.RawAssetChunkBytes)
	chunkIndex := 0
	var offset int64
	for {
		n, readErr := input.Read(buffer)
		if n > 0 {
			chunkBytes := make([]byte, n)
			copy(chunkBytes, buffer[:n])
			if _, err := totalHash.Write(chunkBytes); err != nil {
				return AssetRow{}, 0, err
			}
			if err := writer.Write(ChunkRow{
				AssetID:      assetID,
				RelativePath: file.rel,
				ChunkIndex:   chunkIndex,
				OffsetBytes:  offset,
				SizeBytes:    n,
				SHA256:       digestBytes(chunkBytes),
				Bytes:        chunkBytes,
			}); err != nil {
				return AssetRow{}, 0, err
			}
			chunkIndex++
			offset += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return AssetRow{}, 0, readErr
		}
	}
	if offset != file.info.Size() {
		return AssetRow{}, 0, fmt.Errorf("raw asset size mismatch for %s: read %d bytes, stat reported %d", file.path, offset, file.info.Size())
	}

	extension := strings.ToLower(filepath.Ext(file.path))
	return AssetRow{
		AssetID:        assetID,
		DatasetVersion: cfg.DatasetVersion,
		SchemaVersion:  cfg.SchemaVersion,
		SourceRoot:     file.root,
		DatasetArea:    datasetArea(file.rel),
		RelativePath:   file.rel,
		Path:           file.path,
		URI:            file.path,
		Extension:      extension,
		MediaType:      mediaType(extension),
		SizeBytes:      file.info.Size(),
		SHA256:         digest(totalHash),
		ChunkCount:     chunkIndex,
		ModifiedUnixNS: file.info.ModTime().UnixNano(),
	}, chunkIndex, nil
}

func assetID(cfg config.Config, root string, rel string) string {
	sum := sha256.Sum256([]byte(cfg.DatasetVersion + "\x00" + root + "\x00" + rel))
	return hex.EncodeToString(sum[:])[:24]
}

func digest(hash hash.Hash) string {
	return hex.EncodeToString(hash.Sum(nil))
}

func digestBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func datasetArea(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "raw"
	}
	return parts[0]
}

func mediaType(extension string) string {
	if extension == ".bin" {
		return "application/octet-stream"
	}
	if extension == "" {
		return "application/octet-stream"
	}
	value := mime.TypeByExtension(extension)
	if value == "" {
		return "application/octet-stream"
	}
	return value
}
