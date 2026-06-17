package orchestrator

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tannergabriel/learning-go/advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle/lifecycle"
)

func BuildArtifactManifest(cfg PipelineConfig) ArtifactManifest {
	paths := lifecycle.Paths(cfg.lifecycleConfig())
	runDir := cfg.runDir()
	steps := configuredSteps(cfg)
	artifacts := []Artifact{
		{Name: "run_status_json", Path: filepath.Join(runDir, "run.json")},
		{Name: "run_events_jsonl", Path: filepath.Join(runDir, "events.jsonl")},
		{Name: "run_artifacts_json", Path: filepath.Join(runDir, "artifacts.json")},
	}
	if steps["generate"] || steps["validate"] || steps["features"] || steps["train"] || steps["monitor"] {
		artifacts = append(artifacts, Artifact{Name: "bronze_telemetry_csv", Path: paths.BronzeTelemetryCSV})
	}
	if steps["validate"] || steps["features"] || steps["train"] || steps["monitor"] {
		artifacts = append(artifacts,
			Artifact{Name: "silver_telemetry_csv", Path: paths.SilverTelemetryCSV},
			Artifact{Name: "data_quality_report_json", Path: paths.QualityReportJSON},
		)
	}
	if steps["features"] || steps["train"] || steps["monitor"] {
		artifacts = append(artifacts,
			Artifact{Name: "gold_features_csv", Path: paths.GoldFeaturesCSV},
			Artifact{Name: "gold_events_csv", Path: paths.GoldEventsCSV},
			Artifact{Name: "monitoring_summary_json", Path: paths.MonitoringReportJSON},
		)
	}
	if steps["train"] {
		artifacts = append(artifacts,
			Artifact{Name: "unsafe_maneuver_predictions_csv", Path: filepath.Join(cfg.DataDir, "gold", "unsafe_maneuver_predictions.csv")},
			Artifact{Name: "training_metrics_json", Path: filepath.Join(cfg.ModelsDir, "training_metrics.json")},
			Artifact{Name: "unsafe_maneuver_model_joblib", Path: filepath.Join(cfg.ModelsDir, "unsafe_maneuver_model.joblib")},
			Artifact{Name: "mlflow_tracking_store", Path: cfg.MLflowTrackingURI},
		)
	}
	if steps["train_segmentation"] {
		artifacts = append(artifacts,
			Artifact{Name: "segmentation_lightning_config", Path: cfg.SegmentationConfig},
			Artifact{Name: "segmentation_output_dir", Path: cfg.SegmentationOutputDir},
			Artifact{Name: "segmentation_checkpoint_dir", Path: cfg.SegmentationCheckpointDir},
			Artifact{Name: "segmentation_wandb_dir", Path: cfg.SegmentationWandbDir},
			Artifact{Name: "mlflow_tracking_store", Path: cfg.MLflowTrackingURI},
		)
	}
	if cfg.PerceptionZip != "" {
		artifacts = append(artifacts, Artifact{Name: "perception_dataset_zip", Path: cfg.PerceptionZip})
	}
	if cfg.PerceptionManifest != "" {
		artifacts = append(artifacts, Artifact{Name: "perception_manifest_jsonl", Path: cfg.PerceptionManifest})
	}
	if cfg.PerceptionSummary != "" {
		artifacts = append(artifacts, Artifact{Name: "perception_summary_json", Path: cfg.PerceptionSummary})
	}
	for _, target := range cfg.DVCTargets {
		artifacts = append(artifacts, Artifact{Name: "dvc_target", Path: target})
	}
	for index := range artifacts {
		artifacts[index].Exists = artifactExists(artifacts[index].Path)
	}
	return ArtifactManifest{
		RunID:             cfg.RunID,
		MLflowTrackingURI: cfg.MLflowTrackingURI,
		MLflowExperiment:  cfg.MLflowExperiment,
		Artifacts:         artifacts,
	}
}

func artifactExists(path string) bool {
	localPath := strings.TrimPrefix(path, "sqlite:///")
	if strings.Contains(localPath, "://") {
		return false
	}
	_, err := os.Stat(localPath)
	return err == nil
}

func configuredSteps(cfg PipelineConfig) map[string]bool {
	steps := make(map[string]bool, len(cfg.Steps))
	for _, step := range cfg.Steps {
		steps[step] = true
	}
	return steps
}
