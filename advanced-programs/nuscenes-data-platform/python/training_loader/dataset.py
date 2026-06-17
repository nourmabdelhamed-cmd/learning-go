from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

try:
    from torch.utils.data import Dataset
except Exception:  # pragma: no cover - exercised only when torch is unavailable
    class Dataset:  # type: ignore[no-redef]
        pass


REQUIRED_COLUMNS = {
    "sample_id",
    "scene_id",
    "timestamp",
    "lidar_path",
    "cam_front_path",
    "cam_front_left_path",
    "cam_front_right_path",
    "cam_back_path",
    "cam_back_left_path",
    "cam_back_right_path",
    "ego_pose_id",
    "calibration_id",
    "dataset_version",
    "schema_version",
    "calibration_version",
    "transform_graph_version",
}

PARQUET_SENSOR_BYTE_COLUMNS = {
    "lidar_bytes",
    "cam_front_bytes",
    "cam_front_left_bytes",
    "cam_front_right_bytes",
    "cam_back_bytes",
    "cam_back_left_bytes",
    "cam_back_right_bytes",
}

PARQUET_SENSOR_VALIDATION = [
    ("lidar_bytes", "lidar_sha256", "lidar_size_bytes"),
    ("cam_front_bytes", "cam_front_sha256", "cam_front_size_bytes"),
    ("cam_front_left_bytes", "cam_front_left_sha256", "cam_front_left_size_bytes"),
    ("cam_front_right_bytes", "cam_front_right_sha256", "cam_front_right_size_bytes"),
    ("cam_back_bytes", "cam_back_sha256", "cam_back_size_bytes"),
    ("cam_back_left_bytes", "cam_back_left_sha256", "cam_back_left_size_bytes"),
    ("cam_back_right_bytes", "cam_back_right_sha256", "cam_back_right_size_bytes"),
]

PARQUET_REQUIRED_COLUMNS = {
    "sample_id",
    "scene_id",
    "timestamp",
    "dataset_version",
    "schema_version",
    "calibration_version",
    "transform_graph_version",
    "ego_pose_id",
    "calibration_id",
    "lidar_asset_id",
    "lidar_sha256",
    "lidar_size_bytes",
    "cam_front_asset_id",
    "cam_front_sha256",
    "cam_front_size_bytes",
    "cam_front_left_asset_id",
    "cam_front_left_sha256",
    "cam_front_left_size_bytes",
    "cam_front_right_asset_id",
    "cam_front_right_sha256",
    "cam_front_right_size_bytes",
    "cam_back_asset_id",
    "cam_back_sha256",
    "cam_back_size_bytes",
    "cam_back_left_asset_id",
    "cam_back_left_sha256",
    "cam_back_left_size_bytes",
    "cam_back_right_asset_id",
    "cam_back_right_sha256",
    "cam_back_right_size_bytes",
    *PARQUET_SENSOR_BYTE_COLUMNS,
}


def load_manifest(path: str | Path) -> list[dict[str, Any]]:
    manifest_path = Path(path)
    if not manifest_path.exists():
        raise FileNotFoundError(f"manifest not found: {manifest_path}")

    rows: list[dict[str, Any]] = []
    with manifest_path.open("r", encoding="utf-8") as file:
        for line_number, line in enumerate(file, start=1):
            line = line.strip()
            if not line:
                continue
            row = json.loads(line)
            missing = REQUIRED_COLUMNS.difference(row)
            if missing:
                missing_text = ", ".join(sorted(missing))
                raise ValueError(f"manifest row {line_number} missing: {missing_text}")
            rows.append(row)

    if not rows:
        raise ValueError(f"manifest has no rows: {manifest_path}")
    return rows


def load_parquet_manifest(path: str | Path) -> list[dict[str, Any]]:
    manifest_path = Path(path)
    if not manifest_path.exists():
        raise FileNotFoundError(f"parquet manifest not found: {manifest_path}")

    import pyarrow.parquet as pq

    table = pq.read_table(manifest_path)
    missing = PARQUET_REQUIRED_COLUMNS.difference(table.column_names)
    if missing:
        missing_text = ", ".join(sorted(missing))
        raise ValueError(f"parquet manifest missing columns: {missing_text}")

    rows = table.to_pylist()
    if not rows:
        raise ValueError(f"parquet manifest has no rows: {manifest_path}")

    for index, row in enumerate(rows, start=1):
        if row["schema_version"] != "manifest.v2":
            raise ValueError(f"parquet manifest row {index} schema_version={row['schema_version']}, want manifest.v2")
        for key in PARQUET_SENSOR_BYTE_COLUMNS:
            if row[key] is None:
                raise ValueError(f"parquet manifest row {index} missing bytes column: {key}")
            if isinstance(row[key], bytearray):
                row[key] = bytes(row[key])
        for bytes_key, sha_key, size_key in PARQUET_SENSOR_VALIDATION:
            payload = row[bytes_key]
            if len(payload) != row[size_key]:
                raise ValueError(
                    f"parquet manifest row {index} {bytes_key} size mismatch: "
                    f"got {len(payload)}, want {row[size_key]}"
                )
            if hashlib.sha256(payload).hexdigest() != row[sha_key]:
                raise ValueError(f"parquet manifest row {index} {bytes_key} sha mismatch")
    return rows


class NuScenesManifestDataset(Dataset):
    def __init__(self, manifest_path: str | Path, verify_paths: bool = True) -> None:
        self.manifest_path = Path(manifest_path)
        self.rows = load_manifest(self.manifest_path)
        self.verify_paths = verify_paths

    def __len__(self) -> int:
        return len(self.rows)

    def __getitem__(self, index: int) -> dict[str, Any]:
        row = dict(self.rows[index])
        if self.verify_paths:
            for key in [
                "lidar_path",
                "cam_front_path",
                "cam_front_left_path",
                "cam_front_right_path",
                "cam_back_path",
                "cam_back_left_path",
                "cam_back_right_path",
            ]:
                path = Path(row[key])
                if not path.exists():
                    raise FileNotFoundError(f"{key} does not exist for {row['sample_id']}: {path}")
        return row


class NuScenesParquetManifestDataset(Dataset):
    def __init__(self, manifest_path: str | Path) -> None:
        self.manifest_path = Path(manifest_path)
        self.rows = load_parquet_manifest(self.manifest_path)

    def __len__(self) -> int:
        return len(self.rows)

    def __getitem__(self, index: int) -> dict[str, Any]:
        return dict(self.rows[index])
