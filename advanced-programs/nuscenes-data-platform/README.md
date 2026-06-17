# nuScenes Mini Data Platform in Go

This advanced program implements a local autonomous-driving data platform using real nuScenes mini data.

The goal is not to train a self-driving model. The goal is to show a production-style data path where raw multi-modal autonomous-driving files become a byte-verifiable Parquet lake and a training-ready manifest:

```text
real nuScenes archives
  -> extracted raw data
  -> Go sample inspection
  -> Kafka-shaped event log
  -> bronze byte-for-byte Parquet lake
  -> normalized metadata/features Parquet lake
  -> DuckDB queries
  -> path training manifest
  -> Parquet training dataset
  -> PyTorch Dataset loader
  -> tiny BEVFusion-style PyTorch trainer
```

Local demos use real data only. The tiny fake fixture under `tests/fixtures/` is only for CI and fast tests.

## Data Contract

Local ingestion is complete at the raw-file boundary:

- Every configured raw file is inventoried in `lake/bronze/raw_assets.parquet`.
- Every configured raw file is chunked into ordered binary rows in `lake/bronze/raw_asset_chunks/part-*.parquet`.
- Each raw asset row stores source path, relative path, size, chunk count, media type, and SHA-256.
- Every nuScenes metadata JSON row is preserved generically in `lake/bronze/metadata_records.parquet`.
- Every `sample_data` row is normalized in `lake/metadata/sample_data.parquet`, including sweeps, radar rows, non-keyframes, and keyframes.
- Curated tables and the v1 manifest stay path-based for efficient ML loading, but manifest generation verifies each sensor path exists in the bronze raw asset inventory.
- The v2 Parquet training dataset reconstructs LiDAR and camera bytes from `lake/bronze/raw_asset_chunks/part-*.parquet`, verifies them against `raw_assets.parquet`, and stores one ML-ready row per sample.

Camera image pixels, map JSON files, CAN JSON files, and raw point-cloud files are therefore represented byte-for-byte in the lake. LiDAR keyframe point clouds are also parsed into a typed point table for query/demo features.

For the dataset structure, nuScenes table relationships, lake layout, and data-management rules, read `docs/dataset-management.md`.

## What It Teaches

This module extends lessons from the main `learning-go` repository:

- structs and JSON tags from `basics/10-Struct`
- interfaces from `basics/11-Interfaces`
- channels and workflow thinking from `basics/13-Concurrency`
- file loading from `basics/15-Files-Directory`
- tests from `basics/18-Testing`
- simple runnable services from `beginner-programs/gRPC-Example`
- event/storage boundaries from `advanced-programs/predictor-go`
- artifact boundaries and Python handoff from `vehicle-lifecycle`

## Study Guide

Before running `make demo`, review these Tour of Go topics. They map directly to the implementation:

- Packages, imports, and exported names: the workflow is split across `config`, `nuscenes`, `events`, `rawassets`, `parquetwriter`, and `workflow`, while `cmd/*` packages stay thin runnable entrypoints.
- Structs and struct tags: metadata, event, manifest, and Parquet rows are modeled as structs with `json` and `parquet` tags that define the external data contract.
- Slices, maps, and `range`: the loader groups rows, builds token indexes, collects sensor files by sample, and emits ordered row slices.
- Methods and pointer receivers: some methods mutate receiver state, such as building lookup indexes on a loaded dataset.
- Interfaces: `events.Publisher` lets ingestion write to the current JSONL mock while preserving a boundary for a later Kafka or Redpanda producer.
- Errors: most functions return `(value, error)` and fail fast when raw data, upstream artifacts, or byte checks are missing.
- `defer`: file handles and writers are closed or flushed reliably during long-running ingestion and manifest generation.
- Generics: typed helpers write and read Parquet rows without duplicating code for every row type.

Also look up these data-platform concepts, which are not covered deeply by the Tour of Go:

- Pipeline pattern: raw files become events, bronze Parquet, metadata/features, manifests, and Python training inputs.
- Adapter/interface pattern: ingestion depends on a publisher interface instead of a concrete event system.
- Data contract: schema versions, required artifacts, paths, sizes, and SHA-256 hashes make outputs verifiable.
- Byte-for-byte bronze storage: raw camera, LiDAR, CAN, map, and metadata assets are inventoried and chunked before derived tables are built.
- Deterministic artifact generation: sorted rows, stable hashes, and explicit stage dependencies make repeated runs easier to inspect.

## Required Local Archives

Place these files at the repository root:

```text
v1.0-mini.tgz
can_bus.zip
nuScenes-map-expansion-v1.3.zip
```

They are ignored by Git. Real extracted data and generated Parquet files are also ignored.

## Setup

```bash
cd advanced-programs/nuscenes-data-platform
make setup
```

`make setup` extracts the archives into:

```text
data/raw/nuscenes/
data/raw/can_bus/
data/raw/maps/
```

## Run The Continuous Workflow

Run everything in order:

```bash
make demo
```

Run one stage at a time:

```bash
make inspect-demo
make ingest-demo
make query-demo
make manifest-demo
make parquet-manifest-demo
make train-loader-demo
make train-parquet-loader-demo
make train-bevfusion-demo
make train-bevfusion-parquet-demo
```

Each downstream stage depends on upstream artifacts:

- `ingest-demo` requires extracted real raw data.
- `query-demo` requires generated Parquet under `lake/`.
- `manifest-demo` requires Parquet metadata and feature tables.
- `train-loader-demo` requires `manifests/training_manifest.jsonl`.
- `train-parquet-loader-demo` requires `lake/training/training_manifest_v2.parquet`.
- `train-bevfusion-demo` requires the v1 manifest and reads sensor file paths from it.
- `train-bevfusion-parquet-demo` requires the v2 Parquet dataset and reads sensor bytes directly from Parquet.

## Outputs

The Go ingestion command writes:

```text
runs/events/events.jsonl
lake/events/events.parquet
lake/bronze/raw_assets.parquet
lake/bronze/raw_asset_chunks/part-*.parquet
lake/bronze/metadata_records.parquet
lake/metadata/scenes.parquet
lake/metadata/samples.parquet
lake/metadata/sample_sensors.parquet
lake/metadata/sample_data.parquet
lake/metadata/ego_poses.parquet
lake/metadata/calibrations.parquet
lake/metadata/annotations.parquet
lake/metadata/maps.parquet
lake/features/can_bus.parquet
lake/features/lidar_points.parquet
```

Manifest generation writes:

```text
manifests/training_manifest.jsonl
```

Parquet manifest generation writes:

```text
lake/training/training_manifest_v2.parquet
```

Each manifest row contains:

```text
sample_id
scene_id
timestamp
lidar_path
cam_front_path
cam_front_left_path
cam_front_right_path
cam_back_path
cam_back_left_path
cam_back_right_path
ego_pose_id
calibration_id
dataset_version
schema_version
calibration_version
transform_graph_version
```

Each v2 Parquet row contains the same sample/version fields plus verified bytes and provenance for `LIDAR_TOP` and the six cameras. For each sensor it stores asset ID, relative path, original path/URI, SHA-256, size, media type, chunk count, and a binary `*_bytes` column. Bronze raw bytes are the source of truth; `features/lidar_points.parquet` is a reproducible validation table decoded from those bronze bytes, while the Python loader/encoder converts verified bytes into tensors at training time.

The Python package also includes a small BEVFusion-style trainer:

```bash
PYTHONPATH=python uv run python -m training_loader.train_bevfusion --manifest manifests/training_manifest.jsonl --epochs 3 --batch-size 2
```

Use the verified Parquet dataset directly with:

```bash
PYTHONPATH=python uv run python -m training_loader.train_bevfusion --parquet-manifest lake/training/training_manifest_v2.parquet --epochs 3 --batch-size 2
```

It is intentionally lightweight for local CPU or Mac MPS runs. It rasterizes `LIDAR_TOP` files into a compact BEV tensor, derives compact six-camera features, fuses both branches in PyTorch, and trains a scene-classification proxy task. The trainer logs params, metrics, debug images, prediction tables, checkpoints, and the PyTorch model to MLflow under `runs/mlflow/mlflow.db` by default.

Open the local MLflow UI with:

```bash
uv run mlflow ui --backend-store-uri sqlite:///runs/mlflow/mlflow.db
```

Register the trained model in the MLflow Model Registry by passing a name:

```bash
PYTHONPATH=python uv run python -m training_loader.train_bevfusion --manifest manifests/training_manifest.jsonl --mlflow-register-name nuscenes_tiny_bevfusion
```

Enable Weights & Biases visual debug output with offline or online mode:

```bash
PYTHONPATH=python uv run python -m training_loader.train_bevfusion --manifest manifests/training_manifest.jsonl --wandb-mode offline
```

W&B receives the same scalar metrics plus visual debugging panels for the LiDAR BEV raster, six-camera feature heatmap, and prediction table. Full BEVFusion detection training would require extending the manifest with detection targets and using a larger model stack.

## Kafka Mock

The default event bus is a Kafka-shaped mock. Ingestion publishes typed events to topics such as:

```text
nuscenes.scene.v1
nuscenes.sample.v1
nuscenes.sensor_file.v1
nuscenes.can_bus.v1
nuscenes.map.v1
nuscenes.lidar_points.v1
nuscenes.raw_asset.v1
```

The mock writes deterministic envelopes to `runs/events/events.jsonl` and `lake/events/events.parquet`. A real Redpanda or Kafka producer can later implement the same publisher interface without changing the ingestion logic.

## Test

```bash
make test
make ci-demo
```

Tests and CI use only the tiny fixture. They do not download, extract, or commit real nuScenes data.

## State Boundaries

Track source, tests, docs, configs, and CI. Do not track:

```text
data/
lake/
manifests/
runs/
.venv/
.uv-cache/
*.duckdb
*.log
```

## Lesson Path

Read `docs/lessons/01-prototype.md` first, then continue through lesson 06. Each lesson ties a Go concept to one runnable stage of the workflow.
