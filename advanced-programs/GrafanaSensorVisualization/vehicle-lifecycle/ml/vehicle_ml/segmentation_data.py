from __future__ import annotations

import json
from dataclasses import dataclass
from io import BytesIO
from pathlib import Path
from typing import Any
from zipfile import ZipFile

import lightning as L
import numpy as np
import torch
from PIL import Image
from torch.utils.data import DataLoader, Dataset


@dataclass(frozen=True)
class SegmentationRecord:
    dataset: str
    frame_id: str
    split: str
    image: str
    mask: str
    width: int
    height: int


class ManifestSegmentationDataset(Dataset[dict[str, Any]]):
    def __init__(
        self,
        records: list[SegmentationRecord],
        image_size: tuple[int, int] = (128, 128),
        num_classes: int = 33,
        mean: tuple[float, float, float] = (0.485, 0.456, 0.406),
        std: tuple[float, float, float] = (0.229, 0.224, 0.225),
    ) -> None:
        if not records:
            raise ValueError("segmentation dataset needs at least one manifest record")
        self.records = records
        self.image_size = image_size
        self.num_classes = num_classes
        self.mean = torch.tensor(mean, dtype=torch.float32).view(3, 1, 1)
        self.std = torch.tensor(std, dtype=torch.float32).view(3, 1, 1)
        self._zip_cache: dict[Path, ZipFile] = {}

    def __len__(self) -> int:
        return len(self.records)

    def __getitem__(self, index: int) -> dict[str, Any]:
        record = self.records[index]
        image = self._read_image(record.image).convert("RGB")
        mask = self._read_image(record.mask).convert("RGB")

        height, width = self.image_size
        image = image.resize((width, height), Image.Resampling.BILINEAR)
        mask = mask.resize((width, height), Image.Resampling.NEAREST)

        image_array = np.asarray(image, dtype=np.float32) / 255.0
        image_tensor = torch.from_numpy(image_array).permute(2, 0, 1)
        image_tensor = (image_tensor - self.mean) / self.std

        mask_array = np.asarray(mask, dtype=np.uint8)
        label_tensor = torch.from_numpy(mask_array[:, :, 2].astype(np.int64))
        if int(label_tensor.max()) >= self.num_classes:
            raise ValueError(
                f"mask label {int(label_tensor.max())} exceeds configured num_classes={self.num_classes}"
            )

        return {
            "image": image_tensor,
            "mask": label_tensor,
            "frame_id": record.frame_id,
            "split": record.split,
        }

    def __getstate__(self) -> dict[str, Any]:
        state = self.__dict__.copy()
        state["_zip_cache"] = {}
        return state

    def _read_image(self, uri: str) -> Image.Image:
        zip_path, entry = parse_zip_uri(uri)
        archive = self._zip_cache.get(zip_path)
        if archive is None:
            archive = ZipFile(zip_path)
            self._zip_cache[zip_path] = archive
        return Image.open(BytesIO(archive.read(entry)))


class CamVidDataModule(L.LightningDataModule):
    def __init__(
        self,
        manifest_path: str = "data/perception/camvid_manifest.jsonl",
        image_size: tuple[int, int] = (128, 128),
        batch_size: int = 4,
        num_workers: int = 0,
        num_classes: int = 33,
        max_train_samples: int | None = 64,
        max_val_samples: int | None = 16,
        max_test_samples: int | None = 16,
        pin_memory: bool = False,
    ) -> None:
        super().__init__()
        self.manifest_path = manifest_path
        self.image_size = image_size
        self.batch_size = batch_size
        self.num_workers = num_workers
        self.num_classes = num_classes
        self.max_train_samples = max_train_samples
        self.max_val_samples = max_val_samples
        self.max_test_samples = max_test_samples
        self.pin_memory = pin_memory

    def setup(self, stage: str | None = None) -> None:
        records = load_manifest(Path(self.manifest_path))
        self.train_records = limit_records(records["train"], self.max_train_samples)
        self.val_records = limit_records(records["val"], self.max_val_samples)
        self.test_records = limit_records(records["test"], self.max_test_samples)

        if stage in (None, "fit"):
            self.train_dataset = self._dataset(self.train_records)
            self.val_dataset = self._dataset(self.val_records)
        if stage in (None, "test"):
            self.test_dataset = self._dataset(self.test_records)

    def train_dataloader(self) -> DataLoader[dict[str, Any]]:
        return self._loader(self.train_dataset, shuffle=True)

    def val_dataloader(self) -> DataLoader[dict[str, Any]]:
        return self._loader(self.val_dataset, shuffle=False)

    def test_dataloader(self) -> DataLoader[dict[str, Any]]:
        return self._loader(self.test_dataset, shuffle=False)

    def _dataset(self, records: list[SegmentationRecord]) -> ManifestSegmentationDataset:
        return ManifestSegmentationDataset(
            records=records,
            image_size=self.image_size,
            num_classes=self.num_classes,
        )

    def _loader(self, dataset: ManifestSegmentationDataset, shuffle: bool) -> DataLoader[dict[str, Any]]:
        return DataLoader(
            dataset,
            batch_size=self.batch_size,
            shuffle=shuffle,
            num_workers=self.num_workers,
            pin_memory=self.pin_memory,
        )


def load_manifest(path: Path) -> dict[str, list[SegmentationRecord]]:
    if not path.exists():
        raise FileNotFoundError(f"manifest not found: {path}")

    records = {"train": [], "val": [], "test": []}
    with path.open() as file:
        for line_number, line in enumerate(file, start=1):
            row = json.loads(line)
            split = row.get("split")
            if split not in records:
                raise ValueError(f"line {line_number}: unsupported split {split!r}")
            records[split].append(SegmentationRecord(**row))

    missing = [split for split, split_records in records.items() if not split_records]
    if missing:
        raise ValueError(f"manifest missing required splits: {', '.join(missing)}")
    return records


def limit_records(records: list[SegmentationRecord], limit: int | None) -> list[SegmentationRecord]:
    if limit is None:
        return records
    return records[:limit]


def parse_zip_uri(uri: str) -> tuple[Path, str]:
    prefix = "zip://"
    if not uri.startswith(prefix) or "!/" not in uri:
        raise ValueError(f"expected zip URI like zip://path.zip!/entry, got {uri!r}")
    zip_path, entry = uri[len(prefix) :].split("!/", 1)
    return Path(zip_path), entry
