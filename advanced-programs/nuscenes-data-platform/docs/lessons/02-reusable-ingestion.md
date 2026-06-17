# 02 Reusable Ingestion

The prototype becomes reusable when parsing, joining, and validation move into packages.

Run:

```bash
make ingest-demo
```

The ingestion command loads real scenes, samples, sensors, ego poses, calibration, annotations, CAN bus streams, map metadata, and LiDAR binary files.

Go concepts:

- package boundaries
- explicit errors
- table-driven validation

Exercise: limit the run with `max_samples` in `configs/nuscenes-mini.local.json` and confirm downstream row counts change.
