package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DatasetVersion          string   `json:"dataset_version"`
	SchemaVersion           string   `json:"schema_version"`
	RawRoot                 string   `json:"raw_root"`
	RawAssetRoots           []string `json:"raw_asset_roots"`
	MetadataDir             string   `json:"metadata_dir"`
	SamplesDir              string   `json:"samples_dir"`
	MapsDir                 string   `json:"maps_dir"`
	CanBusDir               string   `json:"can_bus_dir"`
	MapExpansionDir         string   `json:"map_expansion_dir"`
	LakeDir                 string   `json:"lake_dir"`
	EventLogPath            string   `json:"event_log_path"`
	ManifestPath            string   `json:"manifest_path"`
	ParquetManifestPath     string   `json:"parquet_manifest_path"`
	PathBaseURI             string   `json:"path_base_uri"`
	IncludeCANBus           bool     `json:"include_can_bus"`
	IncludeMaps             bool     `json:"include_maps"`
	IncludeRawAssets        bool     `json:"include_raw_assets"`
	RawAssetChunkBytes      int      `json:"raw_asset_chunk_bytes"`
	MaxScenes               int      `json:"max_scenes"`
	MaxSamples              int      `json:"max_samples"`
	MaxLiDARSamples         int      `json:"max_lidar_samples"`
	MaxLiDARPointsPerSample int      `json:"max_lidar_points_per_sample"`
}

func Load(path string) (Config, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return Config{}, err
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.DatasetVersion == "" {
		c.DatasetVersion = "v1.0-mini"
	}
	if c.SchemaVersion == "" {
		c.SchemaVersion = "manifest.v1"
	}
	if c.RawRoot == "" {
		c.RawRoot = filepath.Join("data", "raw", "nuscenes")
	}
	if c.MetadataDir == "" {
		c.MetadataDir = filepath.Join(c.RawRoot, "v1.0-mini")
	}
	if c.SamplesDir == "" {
		c.SamplesDir = filepath.Join(c.RawRoot, "samples")
	}
	if c.MapsDir == "" {
		c.MapsDir = filepath.Join(c.RawRoot, "maps")
	}
	if c.LakeDir == "" {
		c.LakeDir = "lake"
	}
	if c.EventLogPath == "" {
		c.EventLogPath = filepath.Join("runs", "events", "events.jsonl")
	}
	if c.ManifestPath == "" {
		c.ManifestPath = filepath.Join("manifests", "training_manifest.jsonl")
	}
	if c.ParquetManifestPath == "" {
		c.ParquetManifestPath = filepath.Join(c.LakeDir, "training", "training_manifest_v2.parquet")
	}
	if len(c.RawAssetRoots) == 0 {
		c.RawAssetRoots = c.defaultRawAssetRoots()
	}
	if c.RawAssetChunkBytes == 0 {
		c.RawAssetChunkBytes = 4 * 1024 * 1024
	}
}

func (c Config) Validate() error {
	required := map[string]string{
		"raw_root":              c.RawRoot,
		"metadata_dir":          c.MetadataDir,
		"samples_dir":           c.SamplesDir,
		"lake_dir":              c.LakeDir,
		"event_log_path":        c.EventLogPath,
		"manifest_path":         c.ManifestPath,
		"parquet_manifest_path": c.ParquetManifestPath,
	}
	for field, value := range required {
		if value == "" {
			return fmt.Errorf("config %s must not be empty", field)
		}
	}
	if c.IncludeRawAssets && c.RawAssetChunkBytes <= 0 {
		return fmt.Errorf("config raw_asset_chunk_bytes must be positive")
	}
	return nil
}

func (c Config) RequireRawData() error {
	for _, path := range []string{
		c.RawRoot,
		c.MetadataDir,
		c.SamplesDir,
	} {
		if err := requireDir(path); err != nil {
			return err
		}
	}
	if c.IncludeCANBus {
		if err := requireDir(c.CanBusDir); err != nil {
			return err
		}
	}
	if c.IncludeMaps {
		if err := requireDir(c.MapsDir); err != nil {
			return err
		}
		if c.MapExpansionDir != "" {
			if err := requireDir(c.MapExpansionDir); err != nil {
				return err
			}
		}
	}
	if c.IncludeRawAssets {
		for _, path := range c.RawAssetRoots {
			if err := requireDir(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func requireDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("required input path missing: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("required input path is not a directory: %s", path)
	}
	return nil
}

func (c Config) MetadataPath(name string) string {
	return filepath.Join(c.MetadataDir, name+".json")
}

func (c Config) DatasetPath(filename string) string {
	return filepath.Join(c.RawRoot, filepath.FromSlash(filename))
}

func (c Config) ParquetPath(parts ...string) string {
	all := append([]string{c.LakeDir}, parts...)
	return filepath.Join(all...)
}

func (c Config) defaultRawAssetRoots() []string {
	roots := []string{}
	add := func(path string) {
		if path == "" {
			return
		}
		for _, existing := range roots {
			if existing == path {
				return
			}
		}
		roots = append(roots, path)
	}
	add(c.RawRoot)
	if c.IncludeCANBus {
		add(c.CanBusDir)
	}
	if c.IncludeMaps {
		add(c.MapExpansionDir)
	}
	return roots
}
