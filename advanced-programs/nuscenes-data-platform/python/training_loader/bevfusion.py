from __future__ import annotations

from io import BytesIO
from pathlib import Path
from typing import Any

import numpy as np
import torch
from torch import nn

from .dataset import NuScenesManifestDataset, NuScenesParquetManifestDataset


CAMERA_KEYS = [
    "cam_front_path",
    "cam_front_left_path",
    "cam_front_right_path",
    "cam_back_path",
    "cam_back_left_path",
    "cam_back_right_path",
]

PARQUET_CAMERA_BYTE_KEYS = [
    "cam_front_bytes",
    "cam_front_left_bytes",
    "cam_front_right_bytes",
    "cam_back_bytes",
    "cam_back_left_bytes",
    "cam_back_right_bytes",
]


class BEVFusionManifestDataset(NuScenesManifestDataset):
    """Mac-friendly BEVFusion-style sample loader.

    The dataset keeps the manifest contract path-based, then derives compact
    tensors at training time:
    - LIDAR_TOP raw .bin -> small BEV raster.
    - six camera files -> fixed-size per-camera feature vectors.
    """

    def __init__(
        self,
        manifest_path: str | Path,
        *,
        grid_size: int = 64,
        point_range: tuple[float, float, float, float] = (-50.0, 50.0, -50.0, 50.0),
        verify_paths: bool = True,
    ) -> None:
        super().__init__(manifest_path, verify_paths=verify_paths)
        if grid_size <= 0:
            raise ValueError("grid_size must be positive")
        self.grid_size = grid_size
        self.point_range = point_range
        self.scene_to_index = {
            scene_id: index for index, scene_id in enumerate(sorted({row["scene_id"] for row in self.rows}))
        }

    @property
    def num_classes(self) -> int:
        return max(2, len(self.scene_to_index))

    def __getitem__(self, index: int) -> dict[str, Any]:
        row = super().__getitem__(index)
        lidar_bev = lidar_file_to_bev(
            row["lidar_path"],
            grid_size=self.grid_size,
            point_range=self.point_range,
        )
        camera_features = torch.stack([camera_file_to_features(row[key]) for key in CAMERA_KEYS])
        label = torch.tensor(self.scene_to_index[row["scene_id"]], dtype=torch.long)
        return {
            "sample_id": row["sample_id"],
            "scene_id": row["scene_id"],
            "lidar_bev": lidar_bev,
            "camera_features": camera_features,
            "label": label,
        }


class BEVFusionParquetDataset(NuScenesParquetManifestDataset):
    """BEVFusion-style sample loader for verified Parquet training rows."""

    def __init__(
        self,
        manifest_path: str | Path,
        *,
        grid_size: int = 64,
        point_range: tuple[float, float, float, float] = (-50.0, 50.0, -50.0, 50.0),
    ) -> None:
        super().__init__(manifest_path)
        if grid_size <= 0:
            raise ValueError("grid_size must be positive")
        self.grid_size = grid_size
        self.point_range = point_range
        self.scene_to_index = {
            scene_id: index for index, scene_id in enumerate(sorted({row["scene_id"] for row in self.rows}))
        }

    @property
    def num_classes(self) -> int:
        return max(2, len(self.scene_to_index))

    def __getitem__(self, index: int) -> dict[str, Any]:
        row = super().__getitem__(index)
        lidar_bev = lidar_bytes_to_bev(
            row["lidar_bytes"],
            grid_size=self.grid_size,
            point_range=self.point_range,
        )
        camera_features = torch.stack([camera_bytes_to_features(row[key]) for key in PARQUET_CAMERA_BYTE_KEYS])
        label = torch.tensor(self.scene_to_index[row["scene_id"]], dtype=torch.long)
        return {
            "sample_id": row["sample_id"],
            "scene_id": row["scene_id"],
            "lidar_bev": lidar_bev,
            "camera_features": camera_features,
            "label": label,
        }


def lidar_file_to_bev(
    path: str | Path,
    *,
    grid_size: int = 64,
    point_range: tuple[float, float, float, float] = (-50.0, 50.0, -50.0, 50.0),
) -> torch.Tensor:
    payload = Path(path).read_bytes()
    return lidar_bytes_to_bev(payload, grid_size=grid_size, point_range=point_range)


def lidar_bytes_to_bev(
    payload: bytes,
    *,
    grid_size: int = 64,
    point_range: tuple[float, float, float, float] = (-50.0, 50.0, -50.0, 50.0),
) -> torch.Tensor:
    if len(payload) < 20 or len(payload) % 20 != 0:
        return _bytes_to_bev(payload, grid_size)

    points = np.frombuffer(payload, dtype="<f4").reshape(-1, 5)
    x_min, x_max, y_min, y_max = point_range
    x = points[:, 0]
    y = points[:, 1]
    z = points[:, 2]
    intensity = points[:, 3]
    valid = np.isfinite(points).all(axis=1)
    valid &= (x >= x_min) & (x < x_max) & (y >= y_min) & (y < y_max)
    if not np.any(valid):
        return torch.zeros((3, grid_size, grid_size), dtype=torch.float32)

    x = x[valid]
    y = y[valid]
    z = z[valid]
    intensity = intensity[valid]
    cols = ((x - x_min) / (x_max - x_min) * grid_size).astype(np.int64)
    rows = ((y - y_min) / (y_max - y_min) * grid_size).astype(np.int64)
    cols = np.clip(cols, 0, grid_size - 1)
    rows = np.clip(rows, 0, grid_size - 1)

    count = np.zeros((grid_size, grid_size), dtype=np.float32)
    height_sum = np.zeros_like(count)
    intensity_sum = np.zeros_like(count)
    np.add.at(count, (rows, cols), 1.0)
    np.add.at(height_sum, (rows, cols), z)
    np.add.at(intensity_sum, (rows, cols), intensity)

    occupied = count > 0
    height = np.zeros_like(count)
    mean_intensity = np.zeros_like(count)
    height[occupied] = height_sum[occupied] / count[occupied]
    mean_intensity[occupied] = intensity_sum[occupied] / count[occupied]

    density = np.log1p(count) / np.log(64.0)
    height = np.clip((height + 5.0) / 10.0, 0.0, 1.0)
    mean_intensity = np.clip(mean_intensity, 0.0, 1.0)
    bev = np.stack([density, height, mean_intensity]).astype(np.float32)
    return torch.from_numpy(bev)


def camera_file_to_features(path: str | Path) -> torch.Tensor:
    payload = Path(path).read_bytes()
    return camera_bytes_to_features(payload)


def camera_bytes_to_features(payload: bytes) -> torch.Tensor:
    decoded = _try_image_features(payload)
    if decoded is not None:
        return decoded
    return _byte_features(payload)


def _try_image_features(payload: bytes) -> torch.Tensor | None:
    try:
        from PIL import Image
    except Exception:
        return None

    try:
        with Image.open(BytesIO(payload)) as image:
            pixels = np.asarray(image.convert("RGB"), dtype=np.float32) / 255.0
    except Exception:
        return None

    means = pixels.mean(axis=(0, 1))
    stds = pixels.std(axis=(0, 1))
    mins = pixels.min(axis=(0, 1))
    maxs = pixels.max(axis=(0, 1))
    height, width = pixels.shape[:2]
    shape = np.array([height / 2048.0, width / 2048.0], dtype=np.float32)
    size = np.array([len(payload) / 10_000_000.0, 1.0], dtype=np.float32)
    features = np.concatenate([means, stds, mins, maxs, shape, size]).astype(np.float32)
    return torch.from_numpy(features)


def _byte_features(payload: bytes) -> torch.Tensor:
    if not payload:
        return torch.zeros(16, dtype=torch.float32)
    values = np.frombuffer(payload, dtype=np.uint8)
    histogram = np.bincount(values // 32, minlength=8).astype(np.float32)
    histogram /= max(float(values.size), 1.0)
    stats = np.array(
        [
            values.mean() / 255.0,
            values.std() / 255.0,
            values.min() / 255.0,
            values.max() / 255.0,
            min(values.size / 10_000_000.0, 1.0),
            float(values.size % 251) / 250.0,
            float(values[0]) / 255.0,
            float(values[-1]) / 255.0,
        ],
        dtype=np.float32,
    )
    return torch.from_numpy(np.concatenate([histogram, stats]))


def _bytes_to_bev(payload: bytes, grid_size: int) -> torch.Tensor:
    bev = torch.zeros((3, grid_size, grid_size), dtype=torch.float32)
    if not payload:
        return bev
    values = torch.tensor(list(payload[: grid_size * grid_size]), dtype=torch.float32) / 255.0
    bev[0].view(-1)[: values.numel()] = values
    bev[1].fill_(min(len(payload) / 10_000_000.0, 1.0))
    bev[2].fill_((len(payload) % 251) / 250.0)
    return bev


class TinyBEVFusion(nn.Module):
    """Small fusion network that runs on CPU or Apple MPS."""

    def __init__(
        self,
        *,
        num_classes: int,
        camera_count: int = len(CAMERA_KEYS),
        camera_feature_dim: int = 16,
        bev_channels: int = 3,
    ) -> None:
        super().__init__()
        self.lidar_branch = nn.Sequential(
            nn.Conv2d(bev_channels, 16, kernel_size=3, padding=1),
            nn.ReLU(),
            nn.MaxPool2d(2),
            nn.Conv2d(16, 32, kernel_size=3, padding=1),
            nn.ReLU(),
            nn.AdaptiveAvgPool2d((1, 1)),
            nn.Flatten(),
        )
        self.camera_branch = nn.Sequential(
            nn.Flatten(),
            nn.Linear(camera_count * camera_feature_dim, 64),
            nn.ReLU(),
        )
        self.head = nn.Sequential(
            nn.Linear(32 + 64, 64),
            nn.ReLU(),
            nn.Linear(64, num_classes),
        )

    def forward(self, lidar_bev: torch.Tensor, camera_features: torch.Tensor) -> torch.Tensor:
        lidar_embedding = self.lidar_branch(lidar_bev)
        camera_embedding = self.camera_branch(camera_features)
        return self.head(torch.cat([lidar_embedding, camera_embedding], dim=1))
