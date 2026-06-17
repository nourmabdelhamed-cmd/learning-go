package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Run(ctx context.Context, cfg PipelineConfig, registry StepRegistry) (RunStatus, error) {
	if err := normalizeConfig(&cfg); err != nil {
		return RunStatus{}, err
	}
	if registry == nil {
		return RunStatus{}, fmt.Errorf("step registry must not be nil")
	}
	if err := os.MkdirAll(cfg.runDir(), 0755); err != nil {
		return RunStatus{}, err
	}
	if err := resetRunEvents(cfg); err != nil {
		return RunStatus{}, err
	}

	started := time.Now().UTC()
	run := RunStatus{
		RunID:     cfg.RunID,
		Status:    StatusRunning,
		StartedAt: started,
		Config:    cfg,
		Steps:     make([]StepStatus, 0, len(cfg.Steps)),
	}

	if err := appendEvent(cfg, Event{
		RunID:     cfg.RunID,
		Event:     "run_started",
		Timestamp: started,
		Message:   fmt.Sprintf("steps=%v", cfg.Steps),
	}); err != nil {
		return run, err
	}
	if err := writeRunStatus(cfg, run); err != nil {
		return run, err
	}

	var runErr error
	for _, stepName := range cfg.Steps {
		status := runStep(ctx, cfg, stepName, registry)
		run.Steps = append(run.Steps, status)
		if status.Status == StatusFailed {
			run.Status = StatusFailed
			run.Error = status.Error
			runErr = fmt.Errorf("step %s failed: %s", stepName, status.Error)
			break
		}
		if err := writeRunStatus(cfg, run); err != nil {
			return run, err
		}
	}

	if run.Status != StatusFailed {
		run.Status = StatusSucceeded
	}
	run.FinishedAt = time.Now().UTC()
	run.DurationMS = run.FinishedAt.Sub(run.StartedAt).Milliseconds()
	run.Artifacts = BuildArtifactManifest(cfg)

	if err := writeArtifactManifest(cfg, run.Artifacts); err != nil {
		return run, err
	}
	run.Artifacts = BuildArtifactManifest(cfg)
	if err := writeArtifactManifest(cfg, run.Artifacts); err != nil {
		return run, err
	}
	if err := writeRunStatus(cfg, run); err != nil {
		return run, err
	}
	if err := appendEvent(cfg, Event{
		RunID:     cfg.RunID,
		Event:     "run_" + run.Status,
		Timestamp: run.FinishedAt,
		Message:   fmt.Sprintf("duration_ms=%d", run.DurationMS),
	}); err != nil {
		return run, err
	}
	return run, runErr
}

func runStep(ctx context.Context, cfg PipelineConfig, name string, registry StepRegistry) StepStatus {
	started := time.Now().UTC()
	status := StepStatus{
		Name:      name,
		Status:    StatusRunning,
		StartedAt: started,
	}

	step, ok := registry[name]
	if !ok {
		status.Status = StatusFailed
		status.FinishedAt = time.Now().UTC()
		status.DurationMS = status.FinishedAt.Sub(status.StartedAt).Milliseconds()
		status.Error = fmt.Sprintf("unknown step %q", name)
		_ = appendEvent(cfg, Event{
			RunID:     cfg.RunID,
			Step:      name,
			Event:     "step_failed",
			Timestamp: status.FinishedAt,
			Message:   status.Error,
		})
		return status
	}

	attempts := cfg.Retries + 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		status.Attempts = attempt
		attemptStarted := time.Now().UTC()
		_ = appendEvent(cfg, Event{
			RunID:     cfg.RunID,
			Step:      name,
			Event:     "step_started",
			Attempt:   attempt,
			Timestamp: attemptStarted,
		})

		attemptCtx := ctx
		cancel := func() {}
		if timeout := cfg.stepTimeout(); timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		lastErr = step(attemptCtx, cfg)
		cancel()

		if lastErr == nil {
			status.Status = StatusSucceeded
			status.FinishedAt = time.Now().UTC()
			status.DurationMS = status.FinishedAt.Sub(status.StartedAt).Milliseconds()
			_ = appendEvent(cfg, Event{
				RunID:     cfg.RunID,
				Step:      name,
				Event:     "step_succeeded",
				Attempt:   attempt,
				Timestamp: status.FinishedAt,
				Message:   fmt.Sprintf("duration_ms=%d", status.DurationMS),
			})
			return status
		}

		if attempt < attempts {
			_ = appendEvent(cfg, Event{
				RunID:     cfg.RunID,
				Step:      name,
				Event:     "step_retrying",
				Attempt:   attempt,
				Timestamp: time.Now().UTC(),
				Message:   lastErr.Error(),
			})
		}
	}

	status.Status = StatusFailed
	status.FinishedAt = time.Now().UTC()
	status.DurationMS = status.FinishedAt.Sub(status.StartedAt).Milliseconds()
	status.Error = lastErr.Error()
	_ = appendEvent(cfg, Event{
		RunID:     cfg.RunID,
		Step:      name,
		Event:     "step_failed",
		Attempt:   status.Attempts,
		Timestamp: status.FinishedAt,
		Message:   status.Error,
	})
	return status
}

func appendEvent(cfg PipelineConfig, event Event) error {
	path := filepath.Join(cfg.runDir(), "events.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = file.Write(payload)
	return err
}

func resetRunEvents(cfg PipelineConfig) error {
	return os.WriteFile(filepath.Join(cfg.runDir(), "events.jsonl"), nil, 0644)
}

func writeRunStatus(cfg PipelineConfig, status RunStatus) error {
	return writeJSON(filepath.Join(cfg.runDir(), "run.json"), status)
}

func writeArtifactManifest(cfg PipelineConfig, manifest ArtifactManifest) error {
	return writeJSON(filepath.Join(cfg.runDir(), "artifacts.json"), manifest)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}
