# Vehicle Telemetry Data Pipeline in Go + ML Training in Python

This module extends the Grafana sensor demo with a Go-first data engineering lifecycle and a Python scikit-learn training job for autonomous vehicle telemetry.

The goal is not to build a production self-driving model. The goal is to learn where Go and Python meet in real teams: Go handles reliable ingestion, parsing, validation, feature generation, event detection, and monitoring-ready outputs; Python handles model training and evaluation with standard ML tooling.

## What It Builds

```text
synthetic vehicle telemetry
  -> bronze raw CSV
  -> silver validated CSV + data quality report
  -> gold features + event table + monitoring summary
  -> Python scikit-learn unsafe maneuver model
  -> prediction CSV + model metrics + MLflow run
```

The lifecycle covers five portfolio use cases:

- sensor anomaly detection
- harsh braking and unsafe maneuver prediction
- sensor drift detection
- vehicle health monitoring
- road-condition event classification

## Data Source

The current data source is a synthetic Go simulator, not real autonomous-driving data.

The simulator lives in `lifecycle/simulate.go`. It creates vehicle-shaped telemetry with these columns:

- vehicle and trip identifiers
- timestamped speed, acceleration, yaw rate, steering angle, and brake pressure
- wheel vibration, tire pressure, battery voltage, camera and lidar temperatures
- GPS accuracy and road friction

It also injects repeatable operational patterns:

- sudden braking
- high lateral acceleration and sharp steering
- poor GPS accuracy
- camera/lidar temperature drift
- low tire pressure, weak battery, and high vibration
- dry, wet, rough, and icy road conditions

That keeps the project runnable for Go learning. In a company, the upstream source would usually be vehicle logs, simulation logs, ROS bags, fleet telemetry streams, or exported public autonomous-driving datasets. The downstream data engineering shape stays similar: validate the contract, create feature/event tables, train or score a model, and monitor outputs.

The unsafe maneuver training labels are weak labels produced by the rules in `lifecycle/features.go`. High validation metrics are expected on the synthetic data. Treat the model as a learning artifact for training mechanics and artifact management, not as evidence of real-world driving performance.

## Why Go And Python

In many companies, Python is common for research and advanced modeling. Go is common around the system boundary: ingestion services, streaming processors, validation jobs, feature services, model-serving APIs, and operational tooling.

This project keeps the friction line visible:

- Go owns data movement, quality gates, feature tables, event outputs, and monitoring summaries.
- Python owns model training, model persistence, evaluation metrics, prediction files, and MLflow experiment tracking.
- The model uses scikit-learn `Pipeline(StandardScaler, LogisticRegression)` because the problem is tabular classification, not deep learning.
- Accuracy is secondary. Reproducibility, clear data contracts, and operational outputs are the focus.

## Run The Go Data Pipeline

```bash
cd advanced-programs/GrafanaSensorVisualization/vehicle-lifecycle
go run ./cmd/vehicle-lifecycle all -rows 2000
```

Go writes:

```text
data/bronze/vehicle_telemetry.csv
data/silver/vehicle_telemetry_validated.csv
data/silver/data_quality_report.json
data/gold/vehicle_features.csv
data/gold/events.csv
data/gold/monitoring_summary.json
```

Run the Go lifecycle one stage at a time:

```bash
go run ./cmd/vehicle-lifecycle generate
go run ./cmd/vehicle-lifecycle validate
go run ./cmd/vehicle-lifecycle features
```

## Train With Python And MLflow

```bash
uv run python -m vehicle_ml.train \
  --features data/gold/vehicle_features.csv \
  --models models \
  --predictions data/gold/unsafe_maneuver_predictions.csv \
  --experiment vehicle-telemetry-unsafe-maneuver \
  --tracking-uri sqlite:///mlflow.db
```

Python writes:

```text
data/gold/unsafe_maneuver_predictions.csv
models/unsafe_maneuver_model.joblib
models/training_metrics.json
mlflow.db
mlartifacts/
```

The training job logs parameters, metrics, the prediction CSV, the metrics JSON, the joblib model, and an MLflow sklearn model.

Open the local MLflow UI:

```bash
uv run mlflow ui --backend-store-uri sqlite:///mlflow.db
```

## Run The Full Local Pipeline

Go can orchestrate the local learning workflow:

```bash
go run ./cmd/vehicle-lifecycle pipeline -rows 2000
```

That command runs the Go data pipeline first, then invokes:

```bash
uv run python -m vehicle_ml.train ...
```

This is intentionally local orchestration for learning. In a company, this role is usually handled by Airflow, Dagster, Prefect, Argo, Databricks Jobs, or a similar scheduler.

## Custom Go Orchestrator

The next step is a custom orchestration path. This is not meant to replace Airflow or Dagster in a large production platform. It is meant to make the orchestration mechanics visible while keeping Go responsible for the operational boundary.

```bash
go run ./cmd/vehicle-lifecycle orchestrate -config configs/pipeline.json
```

The config controls the run ID, row count, step order, retries, timeout, data/model directories, and MLflow tracking settings:

```json
{
  "run_id": "local-dev",
  "rows": 2000,
  "steps": ["generate", "validate", "features", "train", "monitor"],
  "retries": 1,
  "timeout_seconds": 300,
  "mlflow_tracking_uri": "sqlite:///mlflow.db"
}
```

The orchestrator writes run state and operational metadata:

```text
runs/local-dev/run.json
runs/local-dev/events.jsonl
runs/local-dev/artifacts.json
```

This custom path teaches the core concepts a company scheduler would provide:

- ordered steps
- run IDs
- retries
- per-step timeouts
- status transitions
- event logs
- artifact manifests
- subprocess handoff from Go to Python training

The practical company boundary stays the same: Go orchestrates and records the lifecycle; Python trains the scikit-learn model and logs MLflow metrics/artifacts. A later Airflow, Dagster, Prefect, or Databricks Jobs version would schedule these same boundaries instead of changing the data and ML responsibilities.

## DVC And Public Perception Data

DVC is used for data versioning, not orchestration. The Go orchestrator still decides which steps run and records the run state. DVC owns large data availability and push/pull behavior.

This module initializes DVC in subdirectory mode and tracks a public CamVid road-scene semantic segmentation archive:

```text
datasets/camvid-bluechannel.zip.dvc
```

The actual archive is ignored by Git:

```text
datasets/camvid-bluechannel.zip
```

The dataset was imported with:

```bash
uv run dvc import-url \
  https://datasets.cms.waikato.ac.nz/ufdl/data/camvid/camvid-bluechannel.zip \
  datasets/camvid-bluechannel.zip
```

Run the perception data path:

```bash
go run ./cmd/vehicle-lifecycle orchestrate -config configs/perception-dvc.json
```

That config runs:

```text
dvc_pull -> perception_manifest
```

The `perception_manifest` step reads the zip file without extracting it, pairs each `.jpg` image with its `.png` blue-channel mask, validates dimensions, and writes:

```text
data/perception/camvid_manifest.jsonl
data/perception/camvid_summary.json
runs/camvid-public-dvc/run.json
runs/camvid-public-dvc/events.jsonl
runs/camvid-public-dvc/artifacts.json
```

The current CamVid manifest contains 700 image-mask pairs split deterministically into 560 train, 70 validation, and 70 test rows.

For team-style push/pull, configure a real DVC remote:

```bash
uv run dvc remote add -d storage s3://your-bucket/vehicle-lifecycle
uv run dvc push datasets/camvid-bluechannel.zip.dvc
```

Then another machine can recover the dataset with:

```bash
git pull
uv run dvc pull datasets/camvid-bluechannel.zip.dvc
```

For the public imported URL itself, use `dvc update` when you explicitly want to refresh from the upstream source:

```bash
uv run dvc update datasets/camvid-bluechannel.zip.dvc
```

The custom orchestrator exposes `dvc_pull`, `dvc_update`, and `dvc_push` as step names. The default perception config does not include `dvc_push` because pushing requires a configured remote.

## Train Semantic Segmentation With Lightning

The perception trainer uses PyTorch Lightning with a LightningCLI-style JSON config. Go still builds the data contract and orchestrates the lifecycle; Python owns the model, optimizer, metrics, checkpointing, and MLflow logging.

The training config is:

```text
configs/segmentation-lightning.json
```

It includes:

- trainer settings
- MLflow logger settings
- checkpoint callback
- model architecture and hyperparameters
- data module settings
- manifest path
- batch size and sample limits

Run the trainer directly:

```bash
uv run python -m vehicle_ml.segmentation_cli fit \
  --config configs/segmentation-lightning.json
```

Run the full Go-orchestrated perception training path:

```bash
go run ./cmd/vehicle-lifecycle orchestrate -config configs/perception-training.json
```

That config runs:

```text
dvc_pull -> perception_manifest -> train_segmentation
```

The segmentation config is also logged to the MLflow run under the `config` artifact path.

The local default intentionally trains a small `tiny_unet` on a bounded subset of CamVid so it is runnable on a laptop CPU. To scale closer to a real perception experiment, edit `configs/segmentation-lightning.json`:

- set `data.max_train_samples`, `data.max_val_samples`, and `data.max_test_samples` to `null`
- increase `trainer.max_epochs`
- increase `data.image_size`
- switch `trainer.accelerator` to `gpu` when appropriate
- optionally switch `model.architecture` to `deeplabv3_resnet50` or `fcn_resnet50`

The trainer logs `train_loss`, `train_miou`, `val_loss`, `val_miou`, and pixel accuracy metrics. Checkpoints are written under:

```text
models/perception/checkpoints/
```

The same training run also uses W&B for visual inspection. The default config keeps W&B in offline mode so the pipeline runs without an account:

```json
{
  "class_path": "lightning.pytorch.loggers.WandbLogger",
  "init_args": {
    "project": "vehicle-perception-segmentation",
    "name": "lightning-tiny-unet-camvid",
    "offline": true,
    "save_dir": "models/perception/wandb"
  }
}
```

The `WandbSegmentationVisualizationCallback` logs a fixed validation batch under:

```text
validation/segmentation_samples
```

Each image includes two interactive mask overlays:

```text
prediction
ground_truth
```

Offline W&B runs are written under:

```text
models/perception/wandb/
```

To upload an offline run after logging in:

```bash
uv run wandb login
uv run wandb sync models/perception/wandb/wandb/offline-run-*
```

For live W&B logging during a full local training run, use the online W&B config:

```bash
uv run wandb login
go run ./cmd/vehicle-lifecycle orchestrate -config configs/perception-training-full-wandb.json
```

That full config uses:

```text
configs/segmentation-lightning-full-wandb.json
```

Compared with the laptop smoke config, it:

- sets W&B `offline` to `false`
- trains on all CamVid train/validation/test rows
- uses `image_size: [256, 256]`
- runs for 10 epochs
- logs 8 validation segmentation samples per epoch
- writes full checkpoints to `models/perception/checkpoints-full/`

If `uv run` cannot access its global cache in a restricted environment, use the already-created virtualenv command:

```bash
.venv/bin/wandb login
```

## Test

```bash
go test ./...
uv run pytest
```

The Go tests verify data validation, streaming CSV parsing, feature labels, orchestration arguments, and artifact generation. The Python tests train a scikit-learn model from the Go feature schema and verify local artifacts plus MLflow run logging.

## Connect Back To Grafana

The existing parent project already has MQTT, Telegraf, InfluxDB, and Grafana. The next natural step is to publish rows from `data/gold/events.csv` or `data/gold/unsafe_maneuver_predictions.csv` to MQTT/InfluxDB, then build Grafana panels for:

- unsafe maneuver rate
- sensor drift count by vehicle
- health issue count by vehicle
- road condition distribution
- model precision, recall, and F1

Generated data and model artifacts are intentionally ignored by Git.

## Real Data Upgrade Path

To move beyond the simulator, add a second ingestion command such as:

```bash
go run ./cmd/vehicle-lifecycle ingest-csv -input path/to/vehicle_logs.csv
```

That command should map external vehicle logs into the `TelemetryRecord` schema, then reuse the same validation, feature, training, and monitoring stages.
