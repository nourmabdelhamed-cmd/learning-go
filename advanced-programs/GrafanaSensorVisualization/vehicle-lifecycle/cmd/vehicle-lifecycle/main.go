package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tannergabriel/learning-go/advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle/lifecycle"
	"github.com/tannergabriel/learning-go/advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle/orchestrator"
)

type trainingOptions struct {
	ModelsDir      string
	ExperimentName string
	TrackingURI    string
	RunName        string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	cfg := lifecycle.DefaultConfig()
	training := defaultTrainingOptions()
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	bindCommonFlags(flags, &cfg)
	bindTrainingFlags(flags, &training)
	orchestrationConfigPath := ""
	if command == "orchestrate" {
		flags.StringVar(&orchestrationConfigPath, "config", filepath.Join("configs", "pipeline.json"), "custom orchestration config file")
	}

	if err := flags.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse flags: %v\n", err)
		os.Exit(2)
	}

	switch command {
	case "all":
		result, err := lifecycle.RunAll(cfg)
		exitIfError(err)
		fmt.Printf("pipeline complete\n")
		fmt.Printf("data source: synthetic Go vehicle telemetry simulator\n")
		fmt.Printf("quality: %d valid / %d total\n", result.QualityReport.ValidRecords, result.QualityReport.TotalRecords)
		fmt.Printf("events: unsafe=%d harsh_braking=%d drift=%d health=%d anomalies=%d\n",
			result.MonitoringSummary.UnsafeManeuvers,
			result.MonitoringSummary.HarshBrakingEvents,
			result.MonitoringSummary.SensorDriftEvents,
			result.MonitoringSummary.VehicleHealthIssues,
			result.MonitoringSummary.SensorAnomalies,
		)
		fmt.Printf("features: %s\n", result.Paths.GoldFeaturesCSV)
	case "generate":
		paths, err := lifecycle.RunGenerate(cfg)
		exitIfError(err)
		fmt.Printf("data source: synthetic Go vehicle telemetry simulator\n")
		fmt.Printf("wrote raw telemetry: %s\n", paths.BronzeTelemetryCSV)
	case "validate":
		report, err := lifecycle.RunValidate(cfg)
		exitIfError(err)
		fmt.Printf("quality: %d valid / %d total\n", report.ValidRecords, report.TotalRecords)
	case "features":
		summary, err := lifecycle.RunFeatures(cfg)
		exitIfError(err)
		fmt.Printf("features complete: unsafe=%d road_conditions=%v\n", summary.UnsafeManeuvers, summary.RoadConditions)
	case "pipeline":
		result, err := lifecycle.RunAll(cfg)
		exitIfError(err)
		fmt.Printf("go pipeline complete: features=%s\n", result.Paths.GoldFeaturesCSV)
		exitIfError(runPythonTraining(cfg, training))
	case "orchestrate":
		runCfg, err := orchestratorConfigFromFlags(flags, orchestrationConfigPath, cfg, training)
		exitIfError(err)
		run, err := orchestrator.Run(context.Background(), runCfg, orchestrator.DefaultRegistry())
		exitIfError(err)
		fmt.Printf("custom orchestration complete: run=%s status=%s\n", run.RunID, run.Status)
		fmt.Printf("run state: %s\n", filepath.Join(runCfg.RunsDir, runCfg.RunID, "run.json"))
		fmt.Printf("events: %s\n", filepath.Join(runCfg.RunsDir, runCfg.RunID, "events.jsonl"))
		fmt.Printf("artifacts: %s\n", filepath.Join(runCfg.RunsDir, runCfg.RunID, "artifacts.json"))
	default:
		printUsage()
		os.Exit(2)
	}
}

func bindCommonFlags(flags *flag.FlagSet, cfg *lifecycle.Config) {
	flags.IntVar(&cfg.Rows, "rows", cfg.Rows, "number of synthetic telemetry rows to generate")
	flags.IntVar(&cfg.Vehicles, "vehicles", cfg.Vehicles, "number of synthetic vehicles")
	flags.Int64Var(&cfg.Seed, "seed", cfg.Seed, "deterministic random seed")
	flags.StringVar(&cfg.DataDir, "data", cfg.DataDir, "data output directory")
}

func bindTrainingFlags(flags *flag.FlagSet, options *trainingOptions) {
	flags.StringVar(&options.ModelsDir, "models", options.ModelsDir, "model output directory")
	flags.StringVar(&options.ExperimentName, "experiment", options.ExperimentName, "MLflow experiment name")
	flags.StringVar(&options.TrackingURI, "tracking-uri", options.TrackingURI, "MLflow tracking URI")
	flags.StringVar(&options.RunName, "run-name", options.RunName, "MLflow run name")
}

func defaultTrainingOptions() trainingOptions {
	return trainingOptions{
		ModelsDir:      "models",
		ExperimentName: "vehicle-telemetry-unsafe-maneuver",
		TrackingURI:    "sqlite:///mlflow.db",
		RunName:        "sklearn-logistic-regression",
	}
}

func orchestratorConfigFromFlags(flags *flag.FlagSet, configPath string, cfg lifecycle.Config, training trainingOptions) (orchestrator.PipelineConfig, error) {
	runCfg, err := orchestrator.LoadConfig(configPath)
	if err != nil {
		return orchestrator.PipelineConfig{}, err
	}

	flags.Visit(func(flag *flag.Flag) {
		switch flag.Name {
		case "rows":
			runCfg.Rows = cfg.Rows
		case "vehicles":
			runCfg.Vehicles = cfg.Vehicles
		case "seed":
			runCfg.Seed = cfg.Seed
		case "data":
			runCfg.DataDir = cfg.DataDir
		case "models":
			runCfg.ModelsDir = training.ModelsDir
		case "experiment":
			runCfg.MLflowExperiment = training.ExperimentName
		case "tracking-uri":
			runCfg.MLflowTrackingURI = training.TrackingURI
		case "run-name":
			runCfg.MLflowRunName = training.RunName
		}
	})
	return runCfg, nil
}

func runPythonTraining(cfg lifecycle.Config, options trainingOptions) error {
	cmd := exec.Command("uv", pythonTrainingArgs(cfg, options)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func pythonTrainingArgs(cfg lifecycle.Config, options trainingOptions) []string {
	return []string{
		"run",
		"python",
		"-m",
		"vehicle_ml.train",
		"--features",
		filepath.Join(cfg.DataDir, "gold", "vehicle_features.csv"),
		"--models",
		options.ModelsDir,
		"--predictions",
		filepath.Join(cfg.DataDir, "gold", "unsafe_maneuver_predictions.csv"),
		"--experiment",
		options.ExperimentName,
		"--tracking-uri",
		options.TrackingURI,
		"--run-name",
		options.RunName,
	}
}

func printUsage() {
	fmt.Println("usage: go run ./cmd/vehicle-lifecycle <all|generate|validate|features|pipeline|orchestrate> [flags]")
	fmt.Println()
	fmt.Println("examples:")
	fmt.Println("  go run ./cmd/vehicle-lifecycle all -rows 2000")
	fmt.Println("  go run ./cmd/vehicle-lifecycle pipeline -rows 2000")
	fmt.Println("  go run ./cmd/vehicle-lifecycle orchestrate -config configs/pipeline.json")
	fmt.Println("  go run ./cmd/vehicle-lifecycle orchestrate -config configs/perception-dvc.json")
	fmt.Println("  go run ./cmd/vehicle-lifecycle orchestrate -config configs/perception-training.json")
	fmt.Println("  go run ./cmd/vehicle-lifecycle generate && go run ./cmd/vehicle-lifecycle validate")
	fmt.Println("  uv run python -m vehicle_ml.train --features data/gold/vehicle_features.csv --models models --predictions data/gold/unsafe_maneuver_predictions.csv --tracking-uri sqlite:///mlflow.db")
	fmt.Println()
	fmt.Println("data source:")
	fmt.Println("  synthetic vehicle telemetry generated in Go; no real vehicle logs are bundled")
}

func exitIfError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
