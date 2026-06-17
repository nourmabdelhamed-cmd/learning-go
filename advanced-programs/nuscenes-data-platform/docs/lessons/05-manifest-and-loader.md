# 05 Manifest And Loader

Training jobs need a compact contract, not all raw metadata. The manifest stays small and path-based, but it is now generated only after the bronze raw asset inventory exists.

Run:

```bash
make manifest-demo
make parquet-manifest-demo
make train-loader-demo
make train-parquet-loader-demo
```

The manifest is generated from Parquet metadata and consumed by a minimal PyTorch-compatible `Dataset`. Manifest generation verifies that each LiDAR and camera path appears in `lake/bronze/raw_assets.parquet`, so training inputs are tied back to byte-level lake provenance.

There are two loading contracts:

- `manifest.v1` writes `manifests/training_manifest.jsonl` and keeps training path-based.
- `manifest.v2` writes `lake/training/training_manifest_v2.parquet`, reconstructs required sensor bytes from `lake/bronze/raw_asset_chunks/`, verifies size and SHA-256 against `raw_assets.parquet`, and lets PyTorch load bytes directly from Parquet.

ML handoff concepts:

- Go owns ingestion, validation, and data contracts
- Python owns loader and modeling-facing code
- the loader proves consumption, not model quality

Reference exercise: add raw asset IDs or SHA-256 values to each manifest row for immutable dataset versioning.
