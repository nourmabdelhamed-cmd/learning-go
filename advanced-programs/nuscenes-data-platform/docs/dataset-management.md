# nuScenes Dataset And Data Management Guide

This document explains the shape of the local nuScenes mini dataset and the data-management contract used by this project.

The short version:

```text
source archives
  -> ignored raw landing zone
  -> bronze byte inventory and chunks
  -> normalized Parquet tables
  -> query features
  -> path training manifest
  -> Parquet training dataset
```

The project treats local real data as operational state, not source code. Git tracks code, configs, docs, tests, and tiny CI fixtures. Git does not track downloaded archives, extracted nuScenes files, generated Parquet, event logs, manifests, caches, or virtual environments.

## Source Archives

The local workflow expects these files at the repository root:

```text
v1.0-mini.tgz
can_bus.zip
nuScenes-map-expansion-v1.3.zip
```

`make setup` extracts them into the module-local raw landing area:

```text
advanced-programs/nuscenes-data-platform/data/raw/
  nuscenes/
  can_bus/
  maps/
```

Those paths are ignored by Git. The archives are also ignored by Git at the repository root.

## Raw nuScenes Layout

After extraction, the nuScenes mini archive has this high-level structure:

```text
data/raw/nuscenes/
  v1.0-mini/
    scene.json
    sample.json
    sample_data.json
    sample_annotation.json
    ego_pose.json
    calibrated_sensor.json
    sensor.json
    map.json
    log.json
    category.json
    attribute.json
    instance.json
    visibility.json
  samples/
    CAM_FRONT/
    CAM_FRONT_LEFT/
    CAM_FRONT_RIGHT/
    CAM_BACK/
    CAM_BACK_LEFT/
    CAM_BACK_RIGHT/
    LIDAR_TOP/
    RADAR_FRONT/
    RADAR_FRONT_LEFT/
    RADAR_FRONT_RIGHT/
    RADAR_BACK_LEFT/
    RADAR_BACK_RIGHT/
  sweeps/
    CAM_FRONT/
    CAM_FRONT_LEFT/
    CAM_FRONT_RIGHT/
    CAM_BACK/
    CAM_BACK_LEFT/
    CAM_BACK_RIGHT/
    LIDAR_TOP/
    RADAR_FRONT/
    RADAR_FRONT_LEFT/
    RADAR_FRONT_RIGHT/
    RADAR_BACK_LEFT/
    RADAR_BACK_RIGHT/
  maps/
```

The workflow focuses the training handoff on `LIDAR_TOP` plus the six cameras, but the lake preserves all `sample_data` rows and raw files, including radar and sweeps.

## Core Metadata Model

nuScenes metadata is token-oriented. Most JSON rows have a `token` field, and relationships are expressed by storing another row's token.

The most important chain is:

```text
scene
  -> sample
    -> sample_data
      -> ego_pose
      -> calibrated_sensor
        -> sensor
```

What each core table means:

- `scene.json`: driving sequence metadata. A scene contains ordered samples.
- `sample.json`: keyframe timestamps inside a scene. Each sample points to previous and next samples.
- `sample_data.json`: concrete sensor file records. This includes camera images, LiDAR files, radar files, keyframes, and sweeps.
- `sensor.json`: physical sensor identity, channel, and modality.
- `calibrated_sensor.json`: sensor extrinsics and camera intrinsics.
- `ego_pose.json`: vehicle pose at a timestamp.
- `sample_annotation.json`: object labels and 3D boxes associated with samples.
- `map.json`: map file references and log-token mapping.
- `log.json`: drive/log metadata used to connect scenes and map context.
- `category.json`, `attribute.json`, `instance.json`, `visibility.json`: annotation vocabulary and object-track metadata.

The project keeps two representations of metadata:

- A generic preservation layer in `lake/bronze/metadata_records.parquet`.
- Typed query tables such as `lake/metadata/samples.parquet` and `lake/metadata/sample_data.parquet`.

## Sensor Files

The real dataset contains multiple sensor modalities:

- Six cameras:
  - `CAM_FRONT`
  - `CAM_FRONT_LEFT`
  - `CAM_FRONT_RIGHT`
  - `CAM_BACK`
  - `CAM_BACK_LEFT`
  - `CAM_BACK_RIGHT`
- One top LiDAR:
  - `LIDAR_TOP`
- Five radar channels:
  - `RADAR_FRONT`
  - `RADAR_FRONT_LEFT`
  - `RADAR_FRONT_RIGHT`
  - `RADAR_BACK_LEFT`
  - `RADAR_BACK_RIGHT`

The training contracts intentionally use LiDAR plus six cameras because that is the target ML handoff for this project. The lake still inventories and chunks every configured raw file, including radar files and sweeps.

## CAN Bus And Map Expansion

The optional CAN bus archive is extracted under:

```text
data/raw/can_bus/
```

The ingestion currently creates sample-aligned CAN features in:

```text
lake/features/can_bus.parquet
```

Those rows include nearest sample-level speed, steering angle, longitudinal acceleration, transversal acceleration, and yaw rate.

The map expansion archive is extracted under:

```text
data/raw/maps/
```

Map files are preserved byte-for-byte in the bronze layer and summarized in:

```text
lake/metadata/maps.parquet
```

## Lake Layout

Generated lake artifacts live under:

```text
lake/
  bronze/
  metadata/
  features/
  events/
```

`lake/` is ignored by Git.

### Bronze

Bronze is the lossless source-preservation layer:

```text
lake/bronze/raw_assets.parquet
lake/bronze/raw_asset_chunks/part-*.parquet
lake/bronze/metadata_records.parquet
```

`raw_assets.parquet` has one row per raw file. It records:

- asset ID
- dataset version
- schema version
- source root
- relative path
- absolute/local path
- extension
- media type
- byte size
- SHA-256
- chunk count
- modification timestamp

`raw_asset_chunks/part-*.parquet` stores ordered byte chunks. To reconstruct a file, filter chunks by `asset_id`, order by `chunk_index`, and concatenate `bytes`.

The part-file layout is intentional. A single huge binary Parquet file caused high memory pressure during full local ingestion. Partitioned part files keep the writer bounded and match normal lake storage practice.

`metadata_records.parquet` preserves every row from every source metadata JSON table as:

- table name
- record index
- token when present
- raw compact JSON
- record SHA-256

This means adding a new typed table later does not require re-reading the original archive to recover source metadata.

### Metadata

Metadata tables are normalized, typed tables for analytics and joins:

```text
lake/metadata/scenes.parquet
lake/metadata/samples.parquet
lake/metadata/sample_sensors.parquet
lake/metadata/sample_data.parquet
lake/metadata/ego_poses.parquet
lake/metadata/calibrations.parquet
lake/metadata/annotations.parquet
lake/metadata/maps.parquet
```

Important distinction:

- `sample_sensors.parquet` contains the seven manifest-critical keyframe sensors per sample: `LIDAR_TOP` plus six cameras.
- `sample_data.parquet` contains all `sample_data` rows, including radar, sweeps, keyframes, and non-keyframes.

### Features

Features are derived query or ML-facing tables:

```text
lake/features/can_bus.parquet
lake/features/lidar_points.parquet
```

`lidar_points.parquet` parses keyframe `LIDAR_TOP` files into typed point rows:

```text
sample_id
scene_id
timestamp
lidar_asset_id
lidar_relative_path
lidar_path
lidar_sha256
lidar_size_bytes
parser_version
source_point_count
decoded_point_count
point_index
x
y
z
intensity
ring
```

The raw LiDAR files are preserved byte-for-byte in bronze and remain the source of truth. `lidar_points.parquet` is a derived validation table: it decodes LiDAR from the verified bronze bytes, carries the raw asset ID/hash/path used for association, and can be regenerated from the bronze lake.

### Events

The default event bus is Kafka-shaped but local:

```text
runs/events/events.jsonl
lake/events/events.parquet
```

Each event has a topic, key, schema version, dataset version, source path, event time, and JSON payload. This lets the project demonstrate producer boundaries without requiring a broker for the default workflow.

## Training Contracts

The path-based v1 manifest is:

```text
manifests/training_manifest.jsonl
```

Each row contains:

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

Manifest generation reads from Parquet, not directly from raw JSON. It also verifies that each LiDAR and camera path appears in `raw_assets.parquet`. That keeps the training contract tied to byte-level source provenance.

The lake-backed v2 training dataset is:

```text
lake/training/training_manifest_v2.parquet
```

Each v2 row contains one sample, verified binary payloads for `LIDAR_TOP` and the six cameras, and raw asset provenance columns for each sensor:

```text
*_asset_id
*_relative_path
*_path
*_uri
*_sha256
*_size_bytes
*_media_type
*_chunk_count
*_bytes
```

The v2 builder reconstructs those byte columns from `lake/bronze/raw_asset_chunks/part-*.parquet`, orders chunks by `chunk_index`, and verifies size and SHA-256 against `lake/bronze/raw_assets.parquet` before writing the training dataset. This keeps the PyTorch loader independent of raw filesystem paths. The loader/encoder then turns the verified bytes into tensors at training time; decoded feature tables are reproducible validation outputs, not the canonical source.

## Materialize DuckDB From The Lake

`query-demo` reads Parquet directly through DuckDB. When you want a persistent local `.duckdb` file with physical tables copied from the lake, materialize the Parquet files after `make ingest-demo` and, if needed, `make parquet-manifest-demo`.

Run from `advanced-programs/nuscenes-data-platform`:

```bash
uv run python - <<'PY'
from pathlib import Path

import duckdb

db_path = Path("lake/nuscenes.duckdb")
con = duckdb.connect(str(db_path))

for schema in ["bronze", "metadata", "features", "events", "training"]:
    con.execute(f"CREATE SCHEMA IF NOT EXISTS {schema}")

tables = {
    "bronze.raw_assets": "lake/bronze/raw_assets.parquet",
    "bronze.raw_asset_chunks": "lake/bronze/raw_asset_chunks/*.parquet",
    "bronze.metadata_records": "lake/bronze/metadata_records.parquet",
    "metadata.samples": "lake/metadata/samples.parquet",
    "metadata.sample_sensors": "lake/metadata/sample_sensors.parquet",
    "metadata.sample_data": "lake/metadata/sample_data.parquet",
    "metadata.scenes": "lake/metadata/scenes.parquet",
    "metadata.calibrations": "lake/metadata/calibrations.parquet",
    "metadata.ego_poses": "lake/metadata/ego_poses.parquet",
    "metadata.annotations": "lake/metadata/annotations.parquet",
    "metadata.maps": "lake/metadata/maps.parquet",
    "features.lidar_points": "lake/features/lidar_points.parquet",
    "features.can_bus": "lake/features/can_bus.parquet",
    "events.events": "lake/events/events.parquet",
    "training.training_manifest_v2": "lake/training/training_manifest_v2.parquet",
}

for table, parquet_path in tables.items():
    exists = any(Path().glob(parquet_path)) if "*" in parquet_path else Path(parquet_path).exists()
    if not exists:
        print(f"skip {table}: missing {parquet_path}")
        continue
    con.execute(f"""
        CREATE OR REPLACE TABLE {table} AS
        SELECT * FROM read_parquet('{parquet_path}')
    """)
    rows = con.execute(f"SELECT count(*) FROM {table}").fetchone()[0]
    print(f"{table}: {rows} rows")

con.close()
print(f"wrote {db_path}")
PY
```

This copies data into DuckDB. `bronze.raw_asset_chunks` and `training.training_manifest_v2` include binary sensor payloads, so materializing them duplicates data already stored in Parquet and can make `lake/nuscenes.duckdb` large. For lightweight exploration, prefer the lazy external views in `sql/extern_views.sql`.

## Data Management Practices

### 1. Separate Source, Runtime State, And Code

Tracked by Git:

```text
Go source
Python source
configs
docs
tests
tiny fixtures
CI workflow
lock files
```

Ignored by Git:

```text
data/
lake/
manifests/
runs/
.venv/
.uv-cache/
.pytest_cache/
*.duckdb
*.log
```

This keeps the repository reviewable while allowing large local data workflows.

### 2. Use Real Data Locally, Tiny Fixtures In CI

Local commands use real extracted archives:

```bash
make setup
make ingest-demo
make query-demo
make manifest-demo
make parquet-manifest-demo
make train-loader-demo
make train-parquet-loader-demo
```

CI uses only `tests/fixtures/nuscenes-mini-tiny/`.

The fixture proves parser, join, event, Parquet, manifest, and loader contracts. It does not pretend to be real training data.

### 3. Preserve Raw Bytes Before Deriving Tables

The bronze layer is the recovery boundary. If a derived table is wrong, it can be regenerated from the byte-preserved source files and metadata records.

This is why the project stores both:

- raw bytes in `raw_asset_chunks/`
- typed derived rows in `metadata/` and `features/`

### 4. Store Checksums And Counts

The ingestion summary reports counts such as scenes, samples, raw assets, raw bytes, metadata records, and LiDAR points.

A verified local mini run produced:

```text
scenes=10
samples=404
sample_data_rows=31206
metadata_records=82454
lidar_points=14026208
raw_assets=39068
raw_asset_chunks=39182
raw_asset_bytes=10481928705
events=43126
```

Those numbers are useful as sanity checks when rebuilding the lake.

### 5. Keep Paths Portable

The current config uses local filesystem paths first. The code also keeps URI fields in tables so the same design can later point to object storage paths such as S3.

Do not hard-code absolute machine-specific paths in source code. Put path choices in config.

### 6. Make Stages Explicit

Each stage has a clear upstream dependency:

```text
setup
  -> inspect-demo
  -> ingest-demo
  -> query-demo
  -> manifest-demo
  -> parquet-manifest-demo
  -> train-loader-demo
  -> train-parquet-loader-demo
```

Downstream stages fail with a clear missing-artifact message when upstream artifacts are absent.

### 7. Treat Derived Data As Rebuildable

Generated Parquet, event logs, and manifests are not committed. They can be rebuilt from:

- source archives
- config
- code version

For team or cloud use, the next step would be to publish generated lake artifacts to object storage with a dataset version prefix, not to Git.

### 8. Keep Training Lightweight

The Python loader proves that a manifest can be consumed. It does not run a long training job or require a GPU.

That boundary is deliberate: ingestion quality and dataset contracts should be validated before model training becomes expensive.

## Operational Checks

Use these commands after changes:

```bash
make test
make ci-demo
```

Use these commands for the full local real-data chain:

```bash
make setup
make inspect-demo
make ingest-demo
make query-demo
make manifest-demo
make parquet-manifest-demo
make train-loader-demo
make train-parquet-loader-demo
```

`make demo` runs the full chain in order.

## Relation To Lessons

The lesson documents under `docs/lessons/` explain the staged implementation path. This document is the reference guide for the dataset itself and for the data-management rules that should stay true as the project evolves.
