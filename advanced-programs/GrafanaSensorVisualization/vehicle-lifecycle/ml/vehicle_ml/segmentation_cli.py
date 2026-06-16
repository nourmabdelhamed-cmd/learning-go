from __future__ import annotations

from lightning.pytorch.cli import LightningCLI

from vehicle_ml.segmentation_data import CamVidDataModule
from vehicle_ml.segmentation_model import SegmentationModel


def main() -> None:
    LightningCLI(
        model_class=SegmentationModel,
        datamodule_class=CamVidDataModule,
        seed_everything_default=42,
        save_config_callback=None,
        parser_kwargs={"default_env": True},
    )


if __name__ == "__main__":
    main()
