package nuscenes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tannergabriel/learning-go/advanced-programs/nuscenes-data-platform/internal/config"
)

type MetadataRecordRow struct {
	DatasetVersion string `parquet:"dataset_version"`
	SchemaVersion  string `parquet:"schema_version"`
	TableName      string `parquet:"table_name"`
	RecordIndex    int    `parquet:"record_index"`
	Token          string `parquet:"token"`
	RecordSHA256   string `parquet:"record_sha256"`
	RecordJSON     string `parquet:"record_json"`
}

func MetadataRecordRows(cfg config.Config) ([]MetadataRecordRow, error) {
	entries, err := os.ReadDir(cfg.MetadataDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	rows := []MetadataRecordRow{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		tableName := strings.TrimSuffix(entry.Name(), ".json")
		path := filepath.Join(cfg.MetadataDir, entry.Name())
		payload, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var records []json.RawMessage
		if err := json.Unmarshal(payload, &records); err != nil {
			return nil, fmt.Errorf("parse metadata table %s: %w", path, err)
		}
		for index, record := range records {
			compact := json.RawMessage{}
			if err := compact.UnmarshalJSON(record); err != nil {
				return nil, err
			}
			rows = append(rows, MetadataRecordRow{
				DatasetVersion: cfg.DatasetVersion,
				SchemaVersion:  cfg.SchemaVersion,
				TableName:      tableName,
				RecordIndex:    index,
				Token:          tokenFromRecord(record),
				RecordSHA256:   sha256Hex(record),
				RecordJSON:     string(record),
			})
		}
	}
	return rows, nil
}

func tokenFromRecord(record json.RawMessage) string {
	var decoded map[string]any
	if err := json.Unmarshal(record, &decoded); err != nil {
		return ""
	}
	token, _ := decoded["token"].(string)
	return token
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
