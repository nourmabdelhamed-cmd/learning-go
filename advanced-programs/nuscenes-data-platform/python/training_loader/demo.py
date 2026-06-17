from __future__ import annotations

import argparse

from .dataset import NuScenesManifestDataset, NuScenesParquetManifestDataset


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", default="manifests/training_manifest.jsonl")
    parser.add_argument("--parquet-manifest", default="")
    parser.add_argument("--no-verify-paths", action="store_true")
    args = parser.parse_args()

    if args.parquet_manifest:
        dataset = NuScenesParquetManifestDataset(args.parquet_manifest)
        mode = "parquet"
    else:
        dataset = NuScenesManifestDataset(args.manifest, verify_paths=not args.no_verify_paths)
        mode = "path"
    first = dataset[0]
    print(f"mode={mode}")
    print(f"rows={len(dataset)}")
    print(f"first_sample_id={first['sample_id']}")
    if mode == "parquet":
        print(f"first_lidar_asset_id={first['lidar_asset_id']}")
    else:
        print(f"first_lidar_path={first['lidar_path']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
