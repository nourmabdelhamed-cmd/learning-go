from __future__ import annotations

from pathlib import Path
from typing import Any

import numpy as np
import torch
from lightning import Callback


class MLflowConfigArtifactCallback(Callback):
    def __init__(self, config_path: str, artifact_path: str = "config") -> None:
        self.config_path = config_path
        self.artifact_path = artifact_path
        self._logged = False

    def setup(self, trainer: Any, pl_module: Any, stage: str | None = None) -> None:
        self._log_config(trainer)

    def on_fit_start(self, trainer: Any, pl_module: Any) -> None:
        self._log_config(trainer)

    def _log_config(self, trainer: Any) -> None:
        if self._logged:
            return
        path = Path(self.config_path)
        if not path.exists():
            raise FileNotFoundError(f"segmentation config not found: {path}")

        logger = find_logger(trainer, "MLFlowLogger")
        if logger is None or not hasattr(logger, "experiment") or not hasattr(logger, "run_id"):
            return

        logger.experiment.log_artifact(
            run_id=logger.run_id,
            local_path=str(path),
            artifact_path=self.artifact_path,
        )
        self._logged = True


class WandbSegmentationVisualizationCallback(Callback):
    def __init__(
        self,
        max_images: int = 4,
        every_n_epochs: int = 1,
        mean: tuple[float, float, float] = (0.485, 0.456, 0.406),
        std: tuple[float, float, float] = (0.229, 0.224, 0.225),
    ) -> None:
        self.max_images = max_images
        self.every_n_epochs = every_n_epochs
        self.mean = torch.tensor(mean, dtype=torch.float32).view(1, 3, 1, 1)
        self.std = torch.tensor(std, dtype=torch.float32).view(1, 3, 1, 1)
        self._logged_epochs: set[int] = set()

    def on_validation_batch_end(
        self,
        trainer: Any,
        pl_module: Any,
        outputs: Any,
        batch: dict[str, Any],
        batch_idx: int,
        dataloader_idx: int = 0,
    ) -> None:
        if trainer.sanity_checking or batch_idx != 0:
            return
        epoch = trainer.current_epoch
        if epoch in self._logged_epochs or epoch % self.every_n_epochs != 0:
            return

        logger = find_logger(trainer, "WandbLogger")
        if logger is None:
            return

        import wandb

        images = batch["image"][: self.max_images].to(pl_module.device)
        masks = batch["mask"][: self.max_images].detach().cpu().numpy()
        frame_ids = batch.get("frame_id", [f"sample_{i}" for i in range(images.size(0))])

        with torch.no_grad():
            logits = pl_module(images)
            predictions = torch.argmax(logits, dim=1).detach().cpu().numpy()

        image_arrays = denormalize_images(images.detach().cpu(), self.mean, self.std)
        class_labels = {index: f"class_{index}" for index in range(pl_module.num_classes)}
        wandb_images = []
        for image, prediction, mask, frame_id in zip(image_arrays, predictions, masks, frame_ids):
            wandb_images.append(
                wandb.Image(
                    image,
                    caption=f"epoch={epoch} frame={frame_id}",
                    masks={
                        "prediction": {
                            "mask_data": prediction.astype(np.int32),
                            "class_labels": class_labels,
                        },
                        "ground_truth": {
                            "mask_data": mask.astype(np.int32),
                            "class_labels": class_labels,
                        },
                    },
                )
            )

        logger.experiment.log(
            {
                "validation/segmentation_samples": wandb_images,
                "epoch": epoch,
            }
        )
        self._logged_epochs.add(epoch)


def denormalize_images(
    images: torch.Tensor,
    mean: torch.Tensor,
    std: torch.Tensor,
) -> list[np.ndarray]:
    denormalized = (images * std + mean).clamp(0, 1)
    arrays = denormalized.permute(0, 2, 3, 1).numpy()
    return [(array * 255).astype(np.uint8) for array in arrays]


def find_logger(trainer: Any, class_name: str) -> Any | None:
    loggers = getattr(trainer, "loggers", None)
    if loggers:
        for logger in loggers:
            if logger.__class__.__name__ == class_name:
                return logger

    logger = getattr(trainer, "logger", None)
    if logger is not None and logger.__class__.__name__ == class_name:
        return logger
    return None
