# 01 Prototype: Inspect One Real Sample

Start with one concrete sample before building a pipeline.

Run:

```bash
make setup
make inspect-demo
```

This reads real nuScenes metadata and prints the `LIDAR_TOP` file plus the six camera files for one sample.

Go concepts:

- structs and JSON tags from `basics/10-Struct`
- file paths from `basics/15-Files-Directory`

Exercise: pass a specific sample token to `cmd/inspect-sample` and compare its sensor paths with the nuScenes `sample_data.json` table.
