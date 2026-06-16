package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tannergabriel/learning-go/advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle/lifecycle"
)

func DefaultPipelineConfig() PipelineConfig {
	cfg := lifecycle.DefaultConfig()
	return PipelineConfig{
		RunID:                     "local-" + time.Now().UTC().Format("20060102-150405"),
		Rows:                      cfg.Rows,
		Vehicles:                  cfg.Vehicles,
		Seed:                      cfg.Seed,
		DataDir:                   cfg.DataDir,
		ModelsDir:                 "models",
		RunsDir:                   "runs",
		Steps:                     []string{"generate", "validate", "features", "train", "monitor"},
		Retries:                   1,
		TimeoutSeconds:            300,
		MLflowTrackingURI:         "sqlite:///mlflow.db",
		MLflowExperiment:          "vehicle-telemetry-unsafe-maneuver",
		MLflowRunName:             "custom-go-orchestrator",
		DVCTargets:                []string{},
		PerceptionZip:             filepath.Join("datasets", "camvid-bluechannel.zip"),
		PerceptionManifest:        filepath.Join("data", "perception", "camvid_manifest.jsonl"),
		PerceptionSummary:         filepath.Join("data", "perception", "camvid_summary.json"),
		SegmentationConfig:        filepath.Join("configs", "segmentation-lightning.json"),
		SegmentationOutputDir:     filepath.Join("models", "perception", "lightning"),
		SegmentationCheckpointDir: filepath.Join("models", "perception", "checkpoints"),
		SegmentationWandbDir:      filepath.Join("models", "perception", "wandb"),
	}
}

func LoadConfig(path string) (PipelineConfig, error) {
	cfg := DefaultPipelineConfig()
	if path == "" {
		return cfg, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return PipelineConfig{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return PipelineConfig{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := normalizeConfig(&cfg); err != nil {
		return PipelineConfig{}, err
	}
	return cfg, nil
}

func normalizeConfig(cfg *PipelineConfig) error {
	if cfg.RunID == "" {
		cfg.RunID = DefaultPipelineConfig().RunID
	}
	if cfg.MLflowRunName == "" {
		cfg.MLflowRunName = cfg.RunID
	}
	if cfg.Rows <= 0 {
		return fmt.Errorf("rows must be positive")
	}
	if cfg.Vehicles <= 0 {
		return fmt.Errorf("vehicles must be positive")
	}
	if cfg.DataDir == "" {
		return fmt.Errorf("data_dir must not be empty")
	}
	if cfg.ModelsDir == "" {
		return fmt.Errorf("models_dir must not be empty")
	}
	if cfg.RunsDir == "" {
		return fmt.Errorf("runs_dir must not be empty")
	}
	if len(cfg.Steps) == 0 {
		return fmt.Errorf("steps must not be empty")
	}
	if cfg.Retries < 0 {
		return fmt.Errorf("retries must not be negative")
	}
	if cfg.TimeoutSeconds < 0 {
		return fmt.Errorf("timeout_seconds must not be negative")
	}
	if strings.ContainsAny(cfg.RunID, `/\`) || filepath.Base(cfg.RunID) != cfg.RunID {
		return fmt.Errorf("run_id must be a simple name, got %q", cfg.RunID)
	}
	return nil
}

func (cfg PipelineConfig) lifecycleConfig() lifecycle.Config {
	lifecycleCfg := lifecycle.DefaultConfig()
	lifecycleCfg.Rows = cfg.Rows
	lifecycleCfg.Vehicles = cfg.Vehicles
	lifecycleCfg.Seed = cfg.Seed
	lifecycleCfg.DataDir = cfg.DataDir
	return lifecycleCfg
}

func (cfg PipelineConfig) stepTimeout() time.Duration {
	if cfg.TimeoutSeconds == 0 {
		return 0
	}
	return time.Duration(cfg.TimeoutSeconds) * time.Second
}

func (cfg PipelineConfig) runDir() string {
	return filepath.Join(cfg.RunsDir, cfg.RunID)
}
