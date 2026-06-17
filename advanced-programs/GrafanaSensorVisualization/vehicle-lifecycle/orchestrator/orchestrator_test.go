package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunExecutesConfiguredStepsAndWritesRunFiles(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	cfg.Steps = []string{"generate", "validate", "features"}
	var calls []string
	registry := StepRegistry{
		"generate": func(ctx context.Context, cfg PipelineConfig) error {
			calls = append(calls, "generate")
			return nil
		},
		"validate": func(ctx context.Context, cfg PipelineConfig) error {
			calls = append(calls, "validate")
			return nil
		},
		"features": func(ctx context.Context, cfg PipelineConfig) error {
			calls = append(calls, "features")
			return nil
		},
	}

	run, err := Run(context.Background(), cfg, registry)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if run.Status != StatusSucceeded {
		t.Fatalf("status = %s, want %s", run.Status, StatusSucceeded)
	}
	if !reflect.DeepEqual(calls, cfg.Steps) {
		t.Fatalf("calls = %#v, want %#v", calls, cfg.Steps)
	}
	assertFileExists(t, filepath.Join(cfg.RunsDir, cfg.RunID, "run.json"))
	assertFileExists(t, filepath.Join(cfg.RunsDir, cfg.RunID, "events.jsonl"))
	assertFileExists(t, filepath.Join(cfg.RunsDir, cfg.RunID, "artifacts.json"))
	assertArtifactExists(t, run.Artifacts, "run_artifacts_json")
}

func TestRunRetriesFailedStep(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	cfg.Steps = []string{"train"}
	cfg.Retries = 1
	attempts := 0
	registry := StepRegistry{
		"train": func(ctx context.Context, cfg PipelineConfig) error {
			attempts++
			if attempts == 1 {
				return errors.New("temporary failure")
			}
			return nil
		},
	}

	run, err := Run(context.Background(), cfg, registry)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if run.Steps[0].Attempts != 2 || run.Steps[0].Status != StatusSucceeded {
		t.Fatalf("step status = %#v", run.Steps[0])
	}

	events, err := os.ReadFile(filepath.Join(cfg.RunsDir, cfg.RunID, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(events), "step_retrying") {
		t.Fatalf("events did not include retry: %s", string(events))
	}
}

func TestRunFailsOnUnknownStep(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	cfg.Steps = []string{"missing"}

	run, err := Run(context.Background(), cfg, StepRegistry{})
	if err == nil {
		t.Fatal("Run returned nil error for unknown step")
	}
	if run.Status != StatusFailed {
		t.Fatalf("status = %s, want %s", run.Status, StatusFailed)
	}
	if !strings.Contains(run.Error, "unknown step") {
		t.Fatalf("error = %q", run.Error)
	}
}

func TestRunAppliesStepTimeout(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	cfg.Steps = []string{"slow"}
	cfg.TimeoutSeconds = 1
	registry := StepRegistry{
		"slow": func(ctx context.Context, cfg PipelineConfig) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				return nil
			}
		},
	}

	run, err := Run(context.Background(), cfg, registry)
	if err == nil {
		t.Fatal("Run returned nil error for timed-out step")
	}
	if run.Status != StatusFailed {
		t.Fatalf("status = %s, want %s", run.Status, StatusFailed)
	}
	if !strings.Contains(run.Steps[0].Error, "deadline exceeded") {
		t.Fatalf("step error = %q", run.Steps[0].Error)
	}
}

func TestLoadConfigMergesDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "pipeline.json")
	if err := os.WriteFile(path, []byte(`{"run_id":"config-test","rows":99}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.RunID != "config-test" || cfg.Rows != 99 {
		t.Fatalf("config values not loaded: %#v", cfg)
	}
	if len(cfg.Steps) == 0 || cfg.MLflowTrackingURI == "" {
		t.Fatalf("defaults not preserved: %#v", cfg)
	}
}

func TestBuildArtifactManifestMarksExistingFiles(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	featuresPath := filepath.Join(cfg.DataDir, "gold", "vehicle_features.csv")
	if err := os.MkdirAll(filepath.Dir(featuresPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(featuresPath, []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := BuildArtifactManifest(cfg)
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "gold_features_csv") {
		t.Fatalf("manifest missing gold features artifact: %s", string(payload))
	}

	var found bool
	for _, artifact := range manifest.Artifacts {
		if artifact.Name == "gold_features_csv" {
			found = true
			if !artifact.Exists {
				t.Fatalf("gold_features_csv artifact was not marked existing")
			}
		}
	}
	if !found {
		t.Fatal("gold_features_csv artifact not found")
	}
}

func testConfig(tmp string) PipelineConfig {
	cfg := DefaultPipelineConfig()
	cfg.RunID = "test-run"
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.ModelsDir = filepath.Join(tmp, "models")
	cfg.RunsDir = filepath.Join(tmp, "runs")
	cfg.MLflowTrackingURI = "sqlite:///" + filepath.Join(tmp, "mlflow.db")
	cfg.TimeoutSeconds = 0
	return cfg
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func assertArtifactExists(t *testing.T, manifest ArtifactManifest, name string) {
	t.Helper()
	for _, artifact := range manifest.Artifacts {
		if artifact.Name == name {
			if !artifact.Exists {
				t.Fatalf("artifact %s was not marked existing", name)
			}
			return
		}
	}
	t.Fatalf("artifact %s not found", name)
}
