package orchestrator

import (
	"context"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

type PipelineConfig struct {
	RunID                     string   `json:"run_id"`
	Rows                      int      `json:"rows"`
	Vehicles                  int      `json:"vehicles"`
	Seed                      int64    `json:"seed"`
	DataDir                   string   `json:"data_dir"`
	ModelsDir                 string   `json:"models_dir"`
	RunsDir                   string   `json:"runs_dir"`
	Steps                     []string `json:"steps"`
	Retries                   int      `json:"retries"`
	TimeoutSeconds            int      `json:"timeout_seconds"`
	MLflowTrackingURI         string   `json:"mlflow_tracking_uri"`
	MLflowExperiment          string   `json:"mlflow_experiment"`
	MLflowRunName             string   `json:"mlflow_run_name"`
	DVCTargets                []string `json:"dvc_targets"`
	DVCRemote                 string   `json:"dvc_remote"`
	PerceptionZip             string   `json:"perception_zip"`
	PerceptionManifest        string   `json:"perception_manifest"`
	PerceptionSummary         string   `json:"perception_summary"`
	SegmentationConfig        string   `json:"segmentation_config"`
	SegmentationOutputDir     string   `json:"segmentation_output_dir"`
	SegmentationCheckpointDir string   `json:"segmentation_checkpoint_dir"`
	SegmentationWandbDir      string   `json:"segmentation_wandb_dir"`
}

type StepFunc func(ctx context.Context, cfg PipelineConfig) error

type StepRegistry map[string]StepFunc

type RunStatus struct {
	RunID      string           `json:"run_id"`
	Status     string           `json:"status"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt time.Time        `json:"finished_at"`
	DurationMS int64            `json:"duration_ms"`
	Config     PipelineConfig   `json:"config"`
	Steps      []StepStatus     `json:"steps"`
	Artifacts  ArtifactManifest `json:"artifacts"`
	Error      string           `json:"error,omitempty"`
}

type StepStatus struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Attempts   int       `json:"attempts"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMS int64     `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
}

type Event struct {
	RunID     string    `json:"run_id"`
	Step      string    `json:"step,omitempty"`
	Event     string    `json:"event"`
	Attempt   int       `json:"attempt,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

type ArtifactManifest struct {
	RunID             string     `json:"run_id"`
	MLflowTrackingURI string     `json:"mlflow_tracking_uri"`
	MLflowExperiment  string     `json:"mlflow_experiment"`
	Artifacts         []Artifact `json:"artifacts"`
}

type Artifact struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}
