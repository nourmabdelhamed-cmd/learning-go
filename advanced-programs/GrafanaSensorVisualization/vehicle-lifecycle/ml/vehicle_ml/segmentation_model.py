from __future__ import annotations

from typing import Any

import lightning as L
import torch
from torch import nn
from torchmetrics.classification import MulticlassAccuracy, MulticlassJaccardIndex


class ConvBlock(nn.Module):
    def __init__(self, in_channels: int, out_channels: int) -> None:
        super().__init__()
        self.net = nn.Sequential(
            nn.Conv2d(in_channels, out_channels, kernel_size=3, padding=1, bias=False),
            nn.BatchNorm2d(out_channels),
            nn.ReLU(inplace=True),
            nn.Conv2d(out_channels, out_channels, kernel_size=3, padding=1, bias=False),
            nn.BatchNorm2d(out_channels),
            nn.ReLU(inplace=True),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.net(x)


class TinyUNet(nn.Module):
    def __init__(self, num_classes: int, in_channels: int = 3, base_channels: int = 16) -> None:
        super().__init__()
        self.encoder1 = ConvBlock(in_channels, base_channels)
        self.encoder2 = ConvBlock(base_channels, base_channels * 2)
        self.bottleneck = ConvBlock(base_channels * 2, base_channels * 4)
        self.decoder2 = ConvBlock(base_channels * 4 + base_channels * 2, base_channels * 2)
        self.decoder1 = ConvBlock(base_channels * 2 + base_channels, base_channels)
        self.pool = nn.MaxPool2d(2)
        self.upsample = nn.Upsample(scale_factor=2, mode="bilinear", align_corners=False)
        self.head = nn.Conv2d(base_channels, num_classes, kernel_size=1)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        enc1 = self.encoder1(x)
        enc2 = self.encoder2(self.pool(enc1))
        bottleneck = self.bottleneck(self.pool(enc2))
        dec2 = self.decoder2(torch.cat([self.upsample(bottleneck), enc2], dim=1))
        dec1 = self.decoder1(torch.cat([self.upsample(dec2), enc1], dim=1))
        return self.head(dec1)


class TorchvisionSegmentationWrapper(nn.Module):
    def __init__(self, architecture: str, num_classes: int) -> None:
        super().__init__()
        from torchvision.models.segmentation import deeplabv3_resnet50, fcn_resnet50

        if architecture == "deeplabv3_resnet50":
            self.model = deeplabv3_resnet50(
                weights=None,
                weights_backbone=None,
                num_classes=num_classes,
                aux_loss=False,
            )
        elif architecture == "fcn_resnet50":
            self.model = fcn_resnet50(
                weights=None,
                weights_backbone=None,
                num_classes=num_classes,
                aux_loss=False,
            )
        else:
            raise ValueError(f"unsupported torchvision architecture: {architecture}")

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.model(x)["out"]


class SegmentationModel(L.LightningModule):
    def __init__(
        self,
        num_classes: int = 33,
        architecture: str = "tiny_unet",
        learning_rate: float = 1e-3,
        weight_decay: float = 1e-4,
        ignore_index: int = -100,
        base_channels: int = 16,
    ) -> None:
        super().__init__()
        self.save_hyperparameters()
        self.num_classes = num_classes
        self.learning_rate = learning_rate
        self.weight_decay = weight_decay
        self.ignore_index = ignore_index
        self.model = build_model(architecture, num_classes, base_channels)
        self.loss_fn = nn.CrossEntropyLoss(ignore_index=ignore_index)

        metric_ignore = ignore_index if ignore_index >= 0 else None
        self.train_miou = MulticlassJaccardIndex(num_classes=num_classes, ignore_index=metric_ignore)
        self.val_miou = MulticlassJaccardIndex(num_classes=num_classes, ignore_index=metric_ignore)
        self.test_miou = MulticlassJaccardIndex(num_classes=num_classes, ignore_index=metric_ignore)
        self.val_pixel_accuracy = MulticlassAccuracy(
            num_classes=num_classes,
            average="micro",
            multidim_average="global",
            ignore_index=metric_ignore,
        )
        self.test_pixel_accuracy = MulticlassAccuracy(
            num_classes=num_classes,
            average="micro",
            multidim_average="global",
            ignore_index=metric_ignore,
        )

    def forward(self, images: torch.Tensor) -> torch.Tensor:
        return self.model(images)

    def training_step(self, batch: dict[str, Any], batch_idx: int) -> torch.Tensor:
        loss, preds, masks = self._shared_step(batch)
        batch_size = batch["image"].size(0)
        self.train_miou.update(preds, masks)
        self.log("train_loss", loss, prog_bar=True, on_step=True, on_epoch=True, batch_size=batch_size)
        self.log("train_miou", self.train_miou, prog_bar=True, on_step=False, on_epoch=True, batch_size=batch_size)
        return loss

    def validation_step(self, batch: dict[str, Any], batch_idx: int) -> torch.Tensor:
        loss, preds, masks = self._shared_step(batch)
        batch_size = batch["image"].size(0)
        self.val_miou.update(preds, masks)
        self.val_pixel_accuracy.update(preds, masks)
        self.log("val_loss", loss, prog_bar=True, on_step=False, on_epoch=True, batch_size=batch_size)
        self.log("val_miou", self.val_miou, prog_bar=True, on_step=False, on_epoch=True, batch_size=batch_size)
        self.log(
            "val_pixel_accuracy",
            self.val_pixel_accuracy,
            prog_bar=False,
            on_step=False,
            on_epoch=True,
            batch_size=batch_size,
        )
        return loss

    def test_step(self, batch: dict[str, Any], batch_idx: int) -> torch.Tensor:
        loss, preds, masks = self._shared_step(batch)
        batch_size = batch["image"].size(0)
        self.test_miou.update(preds, masks)
        self.test_pixel_accuracy.update(preds, masks)
        self.log("test_loss", loss, prog_bar=True, on_step=False, on_epoch=True, batch_size=batch_size)
        self.log("test_miou", self.test_miou, prog_bar=True, on_step=False, on_epoch=True, batch_size=batch_size)
        self.log(
            "test_pixel_accuracy",
            self.test_pixel_accuracy,
            prog_bar=False,
            on_step=False,
            on_epoch=True,
            batch_size=batch_size,
        )
        return loss

    def configure_optimizers(self) -> torch.optim.Optimizer:
        return torch.optim.AdamW(
            self.parameters(),
            lr=self.learning_rate,
            weight_decay=self.weight_decay,
        )

    def _shared_step(self, batch: dict[str, Any]) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor]:
        images = batch["image"]
        masks = batch["mask"]
        logits = self(images)
        loss = self.loss_fn(logits, masks)
        preds = torch.argmax(logits, dim=1)
        return loss, preds, masks


def build_model(architecture: str, num_classes: int, base_channels: int) -> nn.Module:
    if architecture == "tiny_unet":
        return TinyUNet(num_classes=num_classes, base_channels=base_channels)
    if architecture in {"deeplabv3_resnet50", "fcn_resnet50"}:
        return TorchvisionSegmentationWrapper(architecture=architecture, num_classes=num_classes)
    raise ValueError(f"unsupported segmentation architecture: {architecture}")
