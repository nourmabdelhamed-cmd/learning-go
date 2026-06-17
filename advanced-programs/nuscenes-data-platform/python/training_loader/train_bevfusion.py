from __future__ import annotations

import argparse
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import torch
from torch import nn
from torch.utils.data import DataLoader, random_split

from .bevfusion import BEVFusionManifestDataset, BEVFusionParquetDataset, TinyBEVFusion


def choose_device(requested: str) -> torch.device:
    if requested != "auto":
        return torch.device(requested)
    if hasattr(torch.backends, "mps") and torch.backends.mps.is_available():
        return torch.device("mps")
    if torch.cuda.is_available():
        return torch.device("cuda")
    return torch.device("cpu")


def main() -> int:
    parser = argparse.ArgumentParser(description="Train a tiny BEVFusion-style model from a nuScenes manifest.")
    parser.add_argument("--manifest", default="manifests/training_manifest.jsonl")
    parser.add_argument("--parquet-manifest", default="")
    parser.add_argument("--epochs", type=int, default=3)
    parser.add_argument("--batch-size", type=int, default=2)
    parser.add_argument("--lr", type=float, default=1e-3)
    parser.add_argument("--grid-size", type=int, default=64)
    parser.add_argument("--device", default="auto", help="auto, cpu, mps, or cuda")
    parser.add_argument("--num-workers", type=int, default=0)
    parser.add_argument("--val-fraction", type=float, default=0.2)
    parser.add_argument("--output", default="runs/bevfusion-tiny/model.pt")
    parser.add_argument("--log-visuals-every", type=int, default=1)
    parser.add_argument("--disable-mlflow", action="store_true")
    parser.add_argument("--mlflow-experiment", default="nuscenes-bevfusion-tiny")
    parser.add_argument("--mlflow-tracking-uri", default="sqlite:///runs/mlflow/mlflow.db")
    parser.add_argument("--mlflow-registry-uri", default="")
    parser.add_argument("--mlflow-run-name", default="")
    parser.add_argument("--mlflow-register-name", default="")
    parser.add_argument("--wandb-mode", choices=["disabled", "offline", "online"], default="disabled")
    parser.add_argument("--wandb-project", default="nuscenes-bevfusion-tiny")
    parser.add_argument("--wandb-entity", default="")
    parser.add_argument("--wandb-run-name", default="")
    args = parser.parse_args()

    if args.epochs <= 0:
        raise ValueError("--epochs must be positive")
    if args.batch_size <= 0:
        raise ValueError("--batch-size must be positive")

    if args.parquet_manifest:
        dataset = BEVFusionParquetDataset(args.parquet_manifest, grid_size=args.grid_size)
    else:
        dataset = BEVFusionManifestDataset(args.manifest, grid_size=args.grid_size)
    train_dataset, val_dataset = split_dataset(dataset, args.val_fraction)
    train_loader = DataLoader(
        train_dataset,
        batch_size=args.batch_size,
        shuffle=True,
        num_workers=args.num_workers,
    )
    val_loader = DataLoader(
        val_dataset,
        batch_size=args.batch_size,
        shuffle=False,
        num_workers=args.num_workers,
    )

    device = choose_device(args.device)
    model = TinyBEVFusion(num_classes=dataset.num_classes).to(device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.lr)
    criterion = nn.CrossEntropyLoss()
    params = tracking_params(args, dataset, device)
    tracker = ExperimentTracker(args, params)

    try:
        print(
            f"device={device} samples={len(dataset)} train={len(train_dataset)} "
            f"val={len(val_dataset)} classes={dataset.num_classes}"
        )
        for epoch in range(1, args.epochs + 1):
            train_loss = train_one_epoch(model, train_loader, optimizer, criterion, device)
            val_loss, val_accuracy = evaluate(model, val_loader, criterion, device)
            metrics = {
                "train/loss": train_loss,
                "val/loss": val_loss,
                "val/accuracy": val_accuracy,
            }
            tracker.log_metrics(metrics, step=epoch)
            if args.log_visuals_every > 0 and epoch % args.log_visuals_every == 0:
                log_debug_visuals(model, val_loader, tracker, device, epoch)
            print(
                f"epoch={epoch} train_loss={train_loss:.4f} "
                f"val_loss={val_loss:.4f} val_accuracy={val_accuracy:.3f}"
            )

        output = Path(args.output)
        output.parent.mkdir(parents=True, exist_ok=True)
        model_cpu = model.to("cpu")
        torch.save(
            {
                "model_state_dict": model_cpu.state_dict(),
                "scene_to_index": dataset.scene_to_index,
                "grid_size": args.grid_size,
                "num_classes": dataset.num_classes,
            },
            output,
        )
        tracker.log_checkpoint_and_model(
            output,
            model_cpu,
            registered_model_name=args.mlflow_register_name or None,
        )
        print(f"checkpoint={output}")
    finally:
        tracker.finish()
    return 0


def split_dataset(
    dataset: BEVFusionManifestDataset | BEVFusionParquetDataset,
    val_fraction: float,
) -> tuple[torch.utils.data.Dataset, torch.utils.data.Dataset]:
    if len(dataset) == 1:
        return dataset, dataset
    val_size = int(round(len(dataset) * val_fraction))
    val_size = min(max(val_size, 1), len(dataset) - 1)
    train_size = len(dataset) - val_size
    generator = torch.Generator().manual_seed(42)
    train_dataset, val_dataset = random_split(dataset, [train_size, val_size], generator=generator)
    return train_dataset, val_dataset


def train_one_epoch(
    model: TinyBEVFusion,
    loader: DataLoader,
    optimizer: torch.optim.Optimizer,
    criterion: nn.Module,
    device: torch.device,
) -> float:
    model.train()
    total_loss = 0.0
    total_items = 0
    for batch in loader:
        lidar_bev = batch["lidar_bev"].to(device)
        camera_features = batch["camera_features"].to(device)
        labels = batch["label"].to(device)

        optimizer.zero_grad(set_to_none=True)
        logits = model(lidar_bev, camera_features)
        loss = criterion(logits, labels)
        loss.backward()
        optimizer.step()

        batch_size = int(labels.shape[0])
        total_loss += float(loss.detach().cpu()) * batch_size
        total_items += batch_size
    return total_loss / max(total_items, 1)


@torch.no_grad()
def evaluate(
    model: TinyBEVFusion,
    loader: DataLoader,
    criterion: nn.Module,
    device: torch.device,
) -> tuple[float, float]:
    model.eval()
    total_loss = 0.0
    total_items = 0
    correct = 0
    for batch in loader:
        lidar_bev = batch["lidar_bev"].to(device)
        camera_features = batch["camera_features"].to(device)
        labels = batch["label"].to(device)
        logits = model(lidar_bev, camera_features)
        loss = criterion(logits, labels)
        predictions = logits.argmax(dim=1)

        batch_size = int(labels.shape[0])
        total_loss += float(loss.detach().cpu()) * batch_size
        total_items += batch_size
        correct += int((predictions == labels).sum().detach().cpu())
    return total_loss / max(total_items, 1), correct / max(total_items, 1)


def tracking_params(
    args: argparse.Namespace,
    dataset: BEVFusionManifestDataset | BEVFusionParquetDataset,
    device: torch.device,
) -> dict[str, Any]:
    return {
        "manifest": args.parquet_manifest or args.manifest,
        "manifest_mode": "parquet" if args.parquet_manifest else "path",
        "epochs": args.epochs,
        "batch_size": args.batch_size,
        "learning_rate": args.lr,
        "grid_size": args.grid_size,
        "device": str(device),
        "num_workers": args.num_workers,
        "val_fraction": args.val_fraction,
        "samples": len(dataset),
        "classes": dataset.num_classes,
        "scene_count": len(dataset.scene_to_index),
        "model": "TinyBEVFusion",
    }


@dataclass
class ExperimentTracker:
    args: argparse.Namespace
    params: dict[str, Any]

    def __post_init__(self) -> None:
        self.mlflow = None
        self.mlflow_pytorch = None
        self.wandb = None
        self.wandb_run = None
        if not self.args.disable_mlflow:
            self._start_mlflow()
        if self.args.wandb_mode != "disabled":
            self._start_wandb()

    def _start_mlflow(self) -> None:
        import mlflow
        import mlflow.pytorch

        if self.args.mlflow_tracking_uri:
            prepare_mlflow_tracking_uri(self.args.mlflow_tracking_uri)
            mlflow.set_tracking_uri(self.args.mlflow_tracking_uri)
        if self.args.mlflow_registry_uri:
            mlflow.set_registry_uri(self.args.mlflow_registry_uri)
        mlflow.set_experiment(self.args.mlflow_experiment)
        mlflow.start_run(run_name=self.args.mlflow_run_name or None)
        mlflow.set_tags(
            {
                "project": "nuscenes-data-platform",
                "task": "bevfusion-style-scene-classification",
            }
        )
        mlflow.log_params(self.params)
        self.mlflow = mlflow
        self.mlflow_pytorch = mlflow.pytorch

    def _start_wandb(self) -> None:
        import wandb

        self.wandb = wandb
        self.wandb_run = wandb.init(
            project=self.args.wandb_project,
            entity=self.args.wandb_entity or None,
            name=self.args.wandb_run_name or self.args.mlflow_run_name or None,
            mode=self.args.wandb_mode,
            config=self.params,
        )

    def log_metrics(self, metrics: dict[str, float], *, step: int) -> None:
        if self.mlflow is not None:
            self.mlflow.log_metrics(metrics, step=step)
        if self.wandb_run is not None:
            self.wandb_run.log(metrics, step=step)

    def log_visuals(
        self,
        *,
        step: int,
        bev_image: np.ndarray,
        camera_feature_image: np.ndarray,
        prediction_rows: list[list[Any]],
    ) -> None:
        columns = ["sample_id", "scene_id", "label", "prediction", "confidence"]
        if self.mlflow is not None:
            self.mlflow.log_image(bev_image, artifact_file=f"debug/epoch_{step:04d}_lidar_bev.png")
            self.mlflow.log_image(
                camera_feature_image,
                artifact_file=f"debug/epoch_{step:04d}_camera_features.png",
            )
            self.mlflow.log_dict(
                {"columns": columns, "rows": prediction_rows},
                artifact_file=f"debug/epoch_{step:04d}_predictions.json",
            )
        if self.wandb_run is not None and self.wandb is not None:
            table = self.wandb.Table(columns=columns, data=prediction_rows)
            self.wandb_run.log(
                {
                    "debug/lidar_bev": self.wandb.Image(bev_image, caption=f"epoch {step} LIDAR BEV"),
                    "debug/camera_features": self.wandb.Image(
                        camera_feature_image,
                        caption=f"epoch {step} six-camera feature heatmap",
                    ),
                    "debug/predictions": table,
                },
                step=step,
            )

    def log_checkpoint_and_model(
        self,
        output: Path,
        model: TinyBEVFusion,
        *,
        registered_model_name: str | None,
    ) -> None:
        if self.mlflow is not None and self.mlflow_pytorch is not None:
            self.mlflow.log_artifact(str(output), artifact_path="checkpoints")
            self.mlflow_pytorch.log_model(
                model,
                name="model",
                registered_model_name=registered_model_name,
                serialization_format="pickle",
            )
        if self.wandb_run is not None and self.wandb is not None:
            artifact = self.wandb.Artifact("tiny-bevfusion-checkpoint", type="model")
            artifact.add_file(str(output))
            self.wandb_run.log_artifact(artifact)

    def finish(self) -> None:
        if self.wandb_run is not None:
            self.wandb_run.finish()
        if self.mlflow is not None:
            self.mlflow.end_run()


@torch.no_grad()
def log_debug_visuals(
    model: TinyBEVFusion,
    loader: DataLoader,
    tracker: ExperimentTracker,
    device: torch.device,
    epoch: int,
) -> None:
    model.eval()
    batch = next(iter(loader), None)
    if batch is None:
        return
    lidar_bev = batch["lidar_bev"].to(device)
    camera_features = batch["camera_features"].to(device)
    labels = batch["label"].to(device)
    logits = model(lidar_bev, camera_features)
    probabilities = torch.softmax(logits, dim=1)
    predictions = probabilities.argmax(dim=1)
    confidences = probabilities.max(dim=1).values
    tracker.log_visuals(
        step=epoch,
        bev_image=bev_to_image(lidar_bev[0]),
        camera_feature_image=camera_features_to_image(camera_features[0]),
        prediction_rows=prediction_rows(batch, labels, predictions, confidences),
    )


def prediction_rows(
    batch: dict[str, Any],
    labels: torch.Tensor,
    predictions: torch.Tensor,
    confidences: torch.Tensor,
) -> list[list[Any]]:
    rows: list[list[Any]] = []
    sample_ids = batch["sample_id"]
    scene_ids = batch["scene_id"]
    for index in range(min(int(labels.shape[0]), 8)):
        rows.append(
            [
                sample_ids[index],
                scene_ids[index],
                int(labels[index].detach().cpu()),
                int(predictions[index].detach().cpu()),
                float(confidences[index].detach().cpu()),
            ]
        )
    return rows


def bev_to_image(bev: torch.Tensor) -> np.ndarray:
    image = bev.detach().cpu().clamp(0, 1).permute(1, 2, 0).numpy()
    return (image * 255.0).astype(np.uint8)


def camera_features_to_image(features: torch.Tensor) -> np.ndarray:
    values = features.detach().cpu().numpy().astype(np.float32)
    values = (values - values.min()) / max(float(values.max() - values.min()), 1e-6)
    upscaled = np.kron(values, np.ones((24, 24), dtype=np.float32))
    image = (upscaled * 255.0).astype(np.uint8)
    return np.stack([image, image, image], axis=-1)


def prepare_mlflow_tracking_uri(uri: str) -> None:
    if uri.startswith("sqlite:///"):
        Path(uri.removeprefix("sqlite:///")).parent.mkdir(parents=True, exist_ok=True)


if __name__ == "__main__":
    raise SystemExit(main())
