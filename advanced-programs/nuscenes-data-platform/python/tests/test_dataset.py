from __future__ import annotations

import json
import hashlib
from pathlib import Path

import numpy as np
import pyarrow as pa
import pyarrow.parquet as pq
import torch

from training_loader import (
    BEVFusionManifestDataset,
    BEVFusionParquetDataset,
    NuScenesManifestDataset,
    TinyBEVFusion,
    load_manifest,
    load_parquet_manifest,
)
from training_loader.train_bevfusion import bev_to_image, camera_features_to_image


def test_load_manifest_and_dataset(tmp_path: Path) -> None:
    lidar = tmp_path / "lidar.bin"
    cam = tmp_path / "cam.jpg"
    lidar.write_bytes(b"lidar")
    cam.write_bytes(b"cam")
    manifest = tmp_path / "manifest.jsonl"
    row = {
        "sample_id": "sample-1",
        "scene_id": "scene-1",
        "timestamp": 123,
        "lidar_path": str(lidar),
        "cam_front_path": str(cam),
        "cam_front_left_path": str(cam),
        "cam_front_right_path": str(cam),
        "cam_back_path": str(cam),
        "cam_back_left_path": str(cam),
        "cam_back_right_path": str(cam),
        "ego_pose_id": "ego-1",
        "calibration_id": "calib-1",
        "dataset_version": "fixture",
        "schema_version": "manifest.v1",
        "calibration_version": "calib-v",
        "transform_graph_version": "tf-v",
    }
    manifest.write_text(json.dumps(row) + "\n", encoding="utf-8")

    rows = load_manifest(manifest)
    assert rows[0]["sample_id"] == "sample-1"

    dataset = NuScenesManifestDataset(manifest)
    assert len(dataset) == 1
    assert dataset[0]["lidar_path"] == str(lidar)


def test_bevfusion_dataset_and_model_accept_manifest_paths(tmp_path: Path) -> None:
    lidar = tmp_path / "lidar.bin"
    cam = tmp_path / "cam.jpg"
    points = np.array(
        [
            [1.0, 1.0, 0.0, 0.5, 0.0],
            [2.0, 1.0, 0.2, 0.7, 0.0],
        ],
        dtype="<f4",
    )
    lidar.write_bytes(points.tobytes())
    cam.write_bytes(b"fixture-camera-bytes")
    manifest = tmp_path / "manifest.jsonl"
    row = {
        "sample_id": "sample-1",
        "scene_id": "scene-1",
        "timestamp": 123,
        "lidar_path": str(lidar),
        "cam_front_path": str(cam),
        "cam_front_left_path": str(cam),
        "cam_front_right_path": str(cam),
        "cam_back_path": str(cam),
        "cam_back_left_path": str(cam),
        "cam_back_right_path": str(cam),
        "ego_pose_id": "ego-1",
        "calibration_id": "calib-1",
        "dataset_version": "fixture",
        "schema_version": "manifest.v1",
        "calibration_version": "calib-v",
        "transform_graph_version": "tf-v",
    }
    manifest.write_text(json.dumps(row) + "\n", encoding="utf-8")

    dataset = BEVFusionManifestDataset(manifest, grid_size=16)
    sample = dataset[0]

    assert sample["lidar_bev"].shape == (3, 16, 16)
    assert sample["camera_features"].shape == (6, 16)
    assert sample["label"].item() == 0

    model = TinyBEVFusion(num_classes=dataset.num_classes)
    logits = model(sample["lidar_bev"].unsqueeze(0), sample["camera_features"].unsqueeze(0))
    assert logits.shape == (1, dataset.num_classes)
    loss = torch.nn.functional.cross_entropy(logits, sample["label"].unsqueeze(0))
    assert torch.isfinite(loss)

    bev_image = bev_to_image(sample["lidar_bev"])
    camera_image = camera_features_to_image(sample["camera_features"])
    assert bev_image.shape == (16, 16, 3)
    assert camera_image.shape == (144, 384, 3)


def test_bevfusion_parquet_dataset_accepts_verified_sensor_bytes(tmp_path: Path) -> None:
    points = np.array(
        [
            [1.0, 1.0, 0.0, 0.5, 0.0],
            [2.0, 1.0, 0.2, 0.7, 0.0],
        ],
        dtype="<f4",
    )
    lidar_bytes = points.tobytes()
    camera_bytes = b"fixture-camera-bytes"
    manifest = tmp_path / "training_manifest_v2.parquet"
    row = {
        "sample_id": "sample-1",
        "scene_id": "scene-1",
        "timestamp": 123,
        "dataset_version": "fixture",
        "schema_version": "manifest.v2",
        "calibration_version": "calib-v",
        "transform_graph_version": "tf-v",
        "ego_pose_id": "ego-1",
        "calibration_id": "calib-1",
        **_sensor_columns("lidar", lidar_bytes),
        **_sensor_columns("cam_front", camera_bytes),
        **_sensor_columns("cam_front_left", camera_bytes),
        **_sensor_columns("cam_front_right", camera_bytes),
        **_sensor_columns("cam_back", camera_bytes),
        **_sensor_columns("cam_back_left", camera_bytes),
        **_sensor_columns("cam_back_right", camera_bytes),
    }
    pq.write_table(pa.Table.from_pylist([row]), manifest)

    rows = load_parquet_manifest(manifest)
    assert rows[0]["schema_version"] == "manifest.v2"

    dataset = BEVFusionParquetDataset(manifest, grid_size=16)
    sample = dataset[0]

    assert sample["lidar_bev"].shape == (3, 16, 16)
    assert sample["camera_features"].shape == (6, 16)
    assert sample["label"].item() == 0


def _sensor_columns(prefix: str, payload: bytes) -> dict[str, object]:
    return {
        f"{prefix}_asset_id": f"{prefix}-asset",
        f"{prefix}_sha256": hashlib.sha256(payload).hexdigest(),
        f"{prefix}_size_bytes": len(payload),
        f"{prefix}_bytes": payload,
    }
