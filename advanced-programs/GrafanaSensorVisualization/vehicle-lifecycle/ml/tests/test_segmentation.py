from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from zipfile import ZipFile

import numpy as np
import torch
from PIL import Image

from vehicle_ml.segmentation_callbacks import denormalize_images
from vehicle_ml.segmentation_data import CamVidDataModule, ManifestSegmentationDataset, load_manifest


def test_manifest_dataset_reads_zip_uris(tmp_path: Path) -> None:
    manifest = write_tiny_segmentation_dataset(tmp_path)
    records = load_manifest(manifest)
    dataset = ManifestSegmentationDataset(records["train"], image_size=(16, 16), num_classes=33)

    sample = dataset[0]

    assert sample["image"].shape == (3, 16, 16)
    assert sample["mask"].shape == (16, 16)
    assert sample["mask"].dtype == torch.long


def test_data_module_builds_train_val_test_loaders(tmp_path: Path) -> None:
    manifest = write_tiny_segmentation_dataset(tmp_path)
    data = CamVidDataModule(
        manifest_path=str(manifest),
        image_size=(16, 16),
        batch_size=2,
        max_train_samples=2,
        max_val_samples=1,
        max_test_samples=1,
    )

    data.setup()
    batch = next(iter(data.train_dataloader()))

    assert batch["image"].shape == (2, 3, 16, 16)
    assert batch["mask"].shape == (2, 16, 16)


def test_denormalize_images_returns_uint8_rgb_arrays() -> None:
    images = torch.zeros((2, 3, 8, 8), dtype=torch.float32)
    mean = torch.tensor((0.5, 0.5, 0.5), dtype=torch.float32).view(1, 3, 1, 1)
    std = torch.tensor((0.25, 0.25, 0.25), dtype=torch.float32).view(1, 3, 1, 1)

    arrays = denormalize_images(images, mean, std)

    assert len(arrays) == 2
    assert arrays[0].shape == (8, 8, 3)
    assert arrays[0].dtype == np.uint8


def test_lightning_cli_trains_one_tiny_epoch(tmp_path: Path) -> None:
    manifest = write_tiny_segmentation_dataset(tmp_path)
    config = tmp_path / "config.json"
    config.write_text(
        json.dumps(
            {
                "seed_everything": 7,
                "trainer": {
                    "accelerator": "cpu",
                    "devices": 1,
                    "max_epochs": 1,
                    "limit_train_batches": 1,
                    "limit_val_batches": 1,
                    "logger": False,
                    "enable_checkpointing": False,
                    "enable_model_summary": False,
                },
                "model": {
                    "architecture": "tiny_unet",
                    "num_classes": 33,
                    "base_channels": 4,
                    "learning_rate": 0.001,
                    "weight_decay": 0.0,
                    "ignore_index": -100,
                },
                "data": {
                    "manifest_path": str(manifest),
                    "image_size": [16, 16],
                    "batch_size": 2,
                    "num_workers": 0,
                    "num_classes": 33,
                    "max_train_samples": 2,
                    "max_val_samples": 1,
                    "max_test_samples": 1,
                    "pin_memory": False,
                },
            }
        ),
        encoding="utf-8",
    )

    result = subprocess.run(
        [sys.executable, "-m", "vehicle_ml.segmentation_cli", "fit", "--config", str(config)],
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 0, result.stderr


def write_tiny_segmentation_dataset(tmp_path: Path) -> Path:
    zip_path = tmp_path / "tiny_camvid.zip"
    manifest_path = tmp_path / "manifest.jsonl"
    rows = []
    splits = ["train", "train", "val", "test"]

    with ZipFile(zip_path, "w") as archive:
        for index, split in enumerate(splits):
            frame_id = f"frame_{index:03d}"
            image_name = f"bluechannel/{frame_id}.jpg"
            mask_name = f"bluechannel/{frame_id}.png"
            image_path = tmp_path / f"{frame_id}.jpg"
            mask_path = tmp_path / f"{frame_id}.png"
            Image.fromarray(np.full((16, 16, 3), 32 + index, dtype=np.uint8)).save(image_path)
            mask = np.zeros((16, 16, 3), dtype=np.uint8)
            mask[:, :, 2] = index % 4
            Image.fromarray(mask).save(mask_path)
            archive.write(image_path, image_name)
            archive.write(mask_path, mask_name)
            rows.append(
                {
                    "dataset": "tiny",
                    "frame_id": frame_id,
                    "split": split,
                    "image": f"zip://{zip_path}!/{image_name}",
                    "mask": f"zip://{zip_path}!/{mask_name}",
                    "width": 16,
                    "height": 16,
                }
            )

    with manifest_path.open("w", encoding="utf-8") as file:
        for row in rows:
            file.write(json.dumps(row) + "\n")
    return manifest_path
