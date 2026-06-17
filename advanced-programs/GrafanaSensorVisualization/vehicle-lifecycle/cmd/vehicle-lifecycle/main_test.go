package main

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tannergabriel/learning-go/advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle/lifecycle"
)

func TestPythonTrainingArgsIncludeMLflowOptions(t *testing.T) {
	cfg := lifecycle.DefaultConfig()
	cfg.DataDir = "tmp-data"
	options := trainingOptions{
		ModelsDir:      "tmp-models",
		ExperimentName: "test-experiment",
		TrackingURI:    "sqlite:///tmp-mlflow.db",
		RunName:        "test-run",
	}

	got := pythonTrainingArgs(cfg, options)
	want := []string{
		"run",
		"python",
		"-m",
		"vehicle_ml.train",
		"--features",
		"tmp-data/gold/vehicle_features.csv",
		"--models",
		"tmp-models",
		"--predictions",
		"tmp-data/gold/unsafe_maneuver_predictions.csv",
		"--experiment",
		"test-experiment",
		"--tracking-uri",
		"sqlite:///tmp-mlflow.db",
		"--run-name",
		"test-run",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestOrchestratorConfigFromFlagsAppliesExplicitOverrides(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "pipeline.json")
	config := `{
  "run_id": "cli-test",
  "rows": 10,
  "vehicles": 2,
  "data_dir": "config-data",
  "models_dir": "config-models",
  "runs_dir": "config-runs",
  "steps": ["generate"],
  "mlflow_tracking_uri": "sqlite:///config.db"
}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := lifecycle.DefaultConfig()
	training := defaultTrainingOptions()
	flags := flag.NewFlagSet("orchestrate", flag.ContinueOnError)
	bindCommonFlags(flags, &cfg)
	bindTrainingFlags(flags, &training)
	if err := flags.Parse([]string{"-rows", "123", "-tracking-uri", "sqlite:///override.db"}); err != nil {
		t.Fatal(err)
	}

	runCfg, err := orchestratorConfigFromFlags(flags, configPath, cfg, training)
	if err != nil {
		t.Fatalf("orchestratorConfigFromFlags returned error: %v", err)
	}
	if runCfg.Rows != 123 {
		t.Fatalf("rows = %d, want 123", runCfg.Rows)
	}
	if runCfg.DataDir != "config-data" {
		t.Fatalf("data_dir = %q, want config value", runCfg.DataDir)
	}
	if runCfg.MLflowTrackingURI != "sqlite:///override.db" {
		t.Fatalf("tracking_uri = %q, want override", runCfg.MLflowTrackingURI)
	}
}
