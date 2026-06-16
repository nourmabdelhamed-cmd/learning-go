package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tannergabriel/learning-go/advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle/lifecycle"
	"github.com/tannergabriel/learning-go/advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle/perception"
)

func DefaultRegistry() StepRegistry {
	return StepRegistry{
		"generate":            RunGenerateStep,
		"validate":            RunValidateStep,
		"features":            RunFeaturesStep,
		"train":               RunPythonTraining,
		"monitor":             RunMonitorStep,
		"dvc_pull":            RunDVCPullStep,
		"dvc_push":            RunDVCPushStep,
		"dvc_update":          RunDVCUpdateStep,
		"perception_manifest": RunPerceptionManifestStep,
		"train_segmentation":  RunSegmentationTraining,
	}
}

func RunGenerateStep(ctx context.Context, cfg PipelineConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := lifecycle.RunGenerate(cfg.lifecycleConfig())
	return err
}

func RunValidateStep(ctx context.Context, cfg PipelineConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := lifecycle.RunValidate(cfg.lifecycleConfig())
	return err
}

func RunFeaturesStep(ctx context.Context, cfg PipelineConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := lifecycle.RunFeatures(cfg.lifecycleConfig())
	return err
}

func RunMonitorStep(ctx context.Context, cfg PipelineConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := lifecycle.Paths(cfg.lifecycleConfig()).MonitoringReportJSON
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("monitoring summary not ready: %w", err)
	}
	return nil
}

func RunPythonTraining(ctx context.Context, cfg PipelineConfig) error {
	cmd := exec.CommandContext(ctx, "uv", pythonTrainingArgs(cfg)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func RunSegmentationTraining(ctx context.Context, cfg PipelineConfig) error {
	if cfg.SegmentationOutputDir != "" {
		if err := os.MkdirAll(cfg.SegmentationOutputDir, 0755); err != nil {
			return err
		}
	}
	if cfg.SegmentationCheckpointDir != "" {
		if err := os.MkdirAll(cfg.SegmentationCheckpointDir, 0755); err != nil {
			return err
		}
	}
	if cfg.SegmentationWandbDir != "" {
		if err := os.MkdirAll(cfg.SegmentationWandbDir, 0755); err != nil {
			return err
		}
	}
	cmd := segmentationTrainingCommand(ctx, cfg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func RunDVCPullStep(ctx context.Context, cfg PipelineConfig) error {
	return runDVC(ctx, cfg, "pull")
}

func RunDVCPushStep(ctx context.Context, cfg PipelineConfig) error {
	return runDVC(ctx, cfg, "push")
}

func RunDVCUpdateStep(ctx context.Context, cfg PipelineConfig) error {
	cmd := dvcCommand(ctx, append([]string{"update"}, cfg.DVCTargets...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func RunPerceptionManifestStep(ctx context.Context, cfg PipelineConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	summary, err := perception.BuildCamVidManifest(cfg.PerceptionZip, cfg.PerceptionManifest, cfg.PerceptionSummary)
	if err != nil {
		return err
	}
	if summary.TotalPairs == 0 {
		return fmt.Errorf("perception manifest has no image-mask pairs")
	}
	return ctx.Err()
}

func runDVC(ctx context.Context, cfg PipelineConfig, command string) error {
	args := []string{command}
	if cfg.DVCRemote != "" {
		args = append(args, "-r", cfg.DVCRemote)
	}
	args = append(args, cfg.DVCTargets...)
	cmd := dvcCommand(ctx, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func pythonTrainingArgs(cfg PipelineConfig) []string {
	return []string{
		"run",
		"python",
		"-m",
		"vehicle_ml.train",
		"--features",
		filepath.Join(cfg.DataDir, "gold", "vehicle_features.csv"),
		"--models",
		cfg.ModelsDir,
		"--predictions",
		filepath.Join(cfg.DataDir, "gold", "unsafe_maneuver_predictions.csv"),
		"--experiment",
		cfg.MLflowExperiment,
		"--tracking-uri",
		cfg.MLflowTrackingURI,
		"--run-name",
		cfg.MLflowRunName,
	}
}

func segmentationTrainingArgs(cfg PipelineConfig) []string {
	return []string{
		"run",
		"python",
		"-m",
		"vehicle_ml.segmentation_cli",
		"fit",
		"--config",
		cfg.SegmentationConfig,
	}
}

func segmentationTrainingCommand(ctx context.Context, cfg PipelineConfig) *exec.Cmd {
	venvPython := filepath.Join(".venv", "bin", "python")
	if _, err := os.Stat(venvPython); err == nil {
		return exec.CommandContext(ctx, venvPython, "-m", "vehicle_ml.segmentation_cli", "fit", "--config", cfg.SegmentationConfig)
	}
	return exec.CommandContext(ctx, "uv", segmentationTrainingArgs(cfg)...)
}

func dvcCommand(ctx context.Context, args ...string) *exec.Cmd {
	venvDVC := filepath.Join(".venv", "bin", "dvc")
	env := append(os.Environ(), "DVC_SITE_CACHE_DIR="+filepath.Join(".dvc", "site-cache"))
	if _, err := os.Stat(venvDVC); err == nil {
		cmd := exec.CommandContext(ctx, venvDVC, args...)
		cmd.Env = env
		return cmd
	}
	cmd := exec.CommandContext(ctx, "uv", append([]string{"run", "dvc"}, args...)...)
	cmd.Env = env
	return cmd
}
