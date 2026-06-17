# Go vs Python for Data Platform Workflows

Audience: Python data engineers who know batch pipelines, Spark, Parquet, and ML handoffs.

This is a slide-style outline for presenting the nuScenes data platform as a concrete comparison. The PySpark snippets are illustrative and are not project dependencies.

## 1. Raw nuScenes Files Become a Data Platform Contract

Claim: the project is not a model-training demo first; it is a data-contract demo.

Proof from this repo:

```text
real nuScenes archives
  -> extracted raw data
  -> Go sample inspection
  -> Kafka-shaped event log
  -> bronze byte-for-byte Parquet lake
  -> normalized metadata/features Parquet lake
  -> DuckDB queries
  -> path training manifest
  -> Parquet training dataset
  -> PyTorch Dataset loader
```

Talking point:

- Go owns deterministic ingestion and contracts.
- Python owns analytics checks, ML loaders, and training-facing ergonomics.
- Parquet is the shared boundary.

## 2. Go CLI Orchestration vs PySpark Job Orchestration

Claim: Go makes the local pipeline a small, typed binary; Spark makes distributed transformations a cluster job.

Go example from `cmd/ingest/main.go`:

```go
cfg, err := config.Load(*configPath)
if err != nil {
	log.Fatal(err)
}
summary, err := workflow.Ingest(context.Background(), cfg)
if err != nil {
	log.Fatal(err)
}
```

Go workflow example from `internal/workflow/ingest.go`:

```go
ds, err := nuscenes.Load(cfg)
bundles, err := ds.Bundles(cfg)
rawAssetRows, rawAssetSummary, err := rawassets.Write(cfg)
lidarSummary, err := WriteLiDARPoints(cfg, bundles, rawAssetRows)
```

Illustrative PySpark equivalent:

```python
from pyspark.sql import SparkSession

spark = SparkSession.builder.appName("nuscenes-ingest").getOrCreate()
cfg = load_config("configs/nuscenes-mini.local.json")

samples = spark.read.json(f"{cfg.metadata_dir}/sample.json")
sample_data = spark.read.json(f"{cfg.metadata_dir}/sample_data.json")
assets = spark.read.format("binaryFile").load(f"{cfg.raw_root}/**/*")
```

Comparison:

- Go is direct for file walking, validation, binary parsing, and CLIs.
- Spark is natural when the same transformation must scale across a cluster.
- In this repo, the local fixture and real mini workflow favor Go for the ingestion boundary.

## 3. Struct Tags vs Spark Schemas

Claim: Go turns row contracts into compiled types; Spark turns them into runtime schemas.

Go example from `internal/workflow/types.go`:

```go
type LiDARPointRow struct {
	SampleID          string  `parquet:"sample_id"`
	SceneID           string  `parquet:"scene_id"`
	Timestamp         int64   `parquet:"timestamp"`
	LiDARAssetID      string  `parquet:"lidar_asset_id"`
	LiDARSHA256       string  `parquet:"lidar_sha256"`
	PointIndex        int64   `parquet:"point_index"`
	X                 float32 `parquet:"x"`
	Y                 float32 `parquet:"y"`
	Z                 float32 `parquet:"z"`
	Intensity         float32 `parquet:"intensity"`
}
```

Go Parquet writer from `internal/parquetwriter/parquetwriter.go`:

```go
func Write[T any](path string, rows []T, options ...parquet.WriterOption) error {
	writerOptions := []parquet.WriterOption{parquet.Compression(&snappy.Codec{})}
	return parquet.WriteFile(path, rows, writerOptions...)
}
```

Illustrative PySpark equivalent:

```python
from pyspark.sql.types import FloatType, LongType, StringType, StructField, StructType

lidar_schema = StructType([
    StructField("sample_id", StringType(), False),
    StructField("scene_id", StringType(), False),
    StructField("timestamp", LongType(), False),
    StructField("lidar_asset_id", StringType(), False),
    StructField("lidar_sha256", StringType(), False),
    StructField("point_index", LongType(), False),
    StructField("x", FloatType(), False),
    StructField("y", FloatType(), False),
    StructField("z", FloatType(), False),
    StructField("intensity", FloatType(), False),
])

lidar_points = spark.createDataFrame(rows, lidar_schema)
lidar_points.write.mode("overwrite").parquet("lake/features/lidar_points.parquet")
```

Comparison:

- Go catches field-name and type mistakes earlier in package code.
- Spark schemas are better when transformations are expressed as relational plans.
- Both need explicit schema discipline; neither saves a platform from vague contracts.

## 4. Events: Interface Boundary vs Kafka/Spark Output

Claim: the event bus is deliberately Kafka-shaped, but local and deterministic first.

Go interface from `internal/events/events.go`:

```go
type Publisher interface {
	Publish(ctx context.Context, topic string, key string, event Envelope) error
	Close() error
	Events() []Envelope
}
```

Go event spec pattern from `internal/workflow/ingest.go`:

```go
return publishRows(ctx, publisher, cfg, rowEventSpec[nuscenes.SampleRow]{
	topic: "nuscenes.sample.v1",
	fields: func(row nuscenes.SampleRow) eventFields {
		return eventFields{
			key:       row.SampleID,
			eventType: "sample",
			sceneID:   row.SceneID,
			sampleID:  row.SampleID,
		}
	},
}, sampleRows)
```

Illustrative PySpark/Kafka equivalent:

```python
from pyspark.sql import functions as F

events = samples.select(
    F.lit("sample").alias("event_type"),
    F.lit("nuscenes.sample.v1").alias("topic"),
    F.col("sample_id").alias("key"),
    F.to_json(F.struct("*")).alias("payload"),
)

events.selectExpr("CAST(key AS STRING)", "payload AS value") \
    .write \
    .format("kafka") \
    .option("kafka.bootstrap.servers", "localhost:9092") \
    .option("topic", "nuscenes.sample.v1") \
    .save()
```

Comparison:

- Go interface: easy to swap JSONL mock for Kafka/Redpanda producer later.
- Spark writer: useful once events are already DataFrames at scale.
- The repo starts with a deterministic JSONL publisher so CI can verify event shape without a broker.

## 5. Byte-Verifiable Bronze Lake

Claim: Go is a practical fit for byte-level ingestion because checksums, chunking, and file IO are plain standard-library work.

Go asset contract from `internal/rawassets/rawassets.go`:

```go
type AssetRow struct {
	AssetID      string `parquet:"asset_id"`
	RelativePath string `parquet:"relative_path"`
	Path         string `parquet:"path"`
	SizeBytes    int64  `parquet:"size_bytes"`
	SHA256       string `parquet:"sha256"`
	ChunkCount   int    `parquet:"chunk_count"`
}

type ChunkRow struct {
	AssetID     string `parquet:"asset_id"`
	ChunkIndex  int    `parquet:"chunk_index"`
	OffsetBytes int64  `parquet:"offset_bytes"`
	SHA256      string `parquet:"sha256"`
	Bytes       []byte `parquet:"bytes"`
}
```

Go streaming read and chunk write:

```go
for {
	n, readErr := input.Read(buffer)
	if n > 0 {
		chunkBytes := make([]byte, n)
		copy(chunkBytes, buffer[:n])
		_ = writer.Write(ChunkRow{
			AssetID:    assetID,
			ChunkIndex: chunkIndex,
			SHA256:     digestBytes(chunkBytes),
			Bytes:      chunkBytes,
		})
	}
	if readErr == io.EOF {
		break
	}
}
```

Illustrative PySpark equivalent:

```python
from pyspark.sql import functions as F

assets = spark.read.format("binaryFile").load(f"{raw_root}/**/*")

bronze_assets = assets.select(
    F.sha2("content", 256).alias("sha256"),
    F.col("path"),
    F.length("content").alias("size_bytes"),
    F.col("modificationTime").alias("modified_at"),
)

bronze_assets.write.mode("overwrite").parquet("lake/bronze/raw_assets.parquet")
```

Comparison:

- Go gives exact control over chunk size, offsets, and memory behavior.
- Spark can hash many files quickly, but binary columns can be expensive if every file is loaded whole.
- This repo uses Go to make raw bytes reconstructable and checksum-verified before ML code consumes them.

## 6. Manifest Generation Is a Data Contract, Not a Convenience File

Claim: the manifest is built only after upstream Parquet artifacts exist and raw asset coverage has been verified.

Go upstream guard from `internal/workflow/artifacts.go`:

```go
func requireArtifacts(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required upstream artifact missing: %s", path)
		}
	}
	return nil
}
```

Go manifest join logic from `internal/workflow/manifest.go`:

```go
samples, _ := parquetwriter.Read[nuscenes.SampleRow](cfg.ParquetPath("metadata", "samples.parquet"))
sensors, _ := parquetwriter.Read[nuscenes.SampleSensorRow](cfg.ParquetPath("metadata", "sample_sensors.parquet"))
assets, _ := parquetwriter.Read[rawassets.AssetRow](cfg.ParquetPath("bronze", "raw_assets.parquet"))

bySample := sampleSensorsBySample(sensors)
assetsByPath := assetsByCleanPath(assets)
```

Illustrative PySpark equivalent:

```python
samples = spark.read.parquet("lake/metadata/samples.parquet")
sensors = spark.read.parquet("lake/metadata/sample_sensors.parquet")
assets = spark.read.parquet("lake/bronze/raw_assets.parquet")

manifest = samples.alias("s") \
    .join(sensors.alias("lidar"), "sample_id") \
    .where("lidar.sensor_channel = 'LIDAR_TOP'") \
    .join(assets.alias("a"), F.col("lidar.path") == F.col("a.path")) \
    .select(
        "sample_id",
        "scene_id",
        "timestamp",
        F.col("lidar.path").alias("lidar_path"),
        F.col("a.sha256").alias("lidar_sha256"),
    )
```

Comparison:

- Go code is explicit and easy to test on a tiny fixture.
- Spark SQL is concise for joins, pivots, and table-scale checks.
- The contract matters more than the language: every training row must trace back to raw asset provenance.

## 7. Parquet Manifest v2 Carries Verified Sensor Bytes

Claim: `manifest.v2` moves beyond paths and stores verified LiDAR/camera bytes plus provenance in one ML-ready row.

Go sensor spec pattern from `internal/workflow/parquet_manifest.go`:

```go
type parquetManifestSensorSpec struct {
	channel string
	set     func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte)
}

var parquetManifestSensorSpecs = []parquetManifestSensorSpec{
	{channel: "LIDAR_TOP", set: func(row *ParquetManifestRow, asset rawassets.AssetRow, payload []byte) {
		row.LiDARAssetID = asset.AssetID
		row.LiDARSHA256 = asset.SHA256
		row.LiDARBytes = payload
	}},
}
```

Go verification helper:

```go
payload := assetBytes[asset.AssetID]
if err := rawassets.VerifyAssetBytes(asset, payload); err != nil {
	return err
}
spec.set(row, asset, payload)
```

Illustrative PySpark equivalent:

```python
from pyspark.sql import functions as F

chunks = spark.read.parquet("lake/bronze/raw_asset_chunks/*.parquet")

ordered = chunks.groupBy("asset_id").agg(
    F.sort_array(F.collect_list(F.struct("chunk_index", "bytes"))).alias("chunks")
)

@F.udf("binary")
def concat_chunks(chunks: list[dict]) -> bytes:
    return b"".join(chunk["bytes"] for chunk in chunks)

asset_bytes = ordered.withColumn("payload", concat_chunks("chunks"))

verified = assets.join(asset_bytes, "asset_id") \
    .where(F.sha2("payload", 256) == F.col("sha256"))
```

Comparison:

- Go keeps the reconstruction and checksum logic close to the binary asset reader.
- Spark can express the validation, but ordered binary reconstruction needs careful handling.
- The repo deliberately uses Go for this step, then hands Python a simple validated row format.

## 8. Python Handoff: Loaders and Training Stay Pythonic

Claim: Go should not replace Python where Python is already the best interface for ML users.

Python loader from `python/training_loader/dataset.py`:

```python
def load_parquet_manifest(path: str | Path) -> list[dict[str, Any]]:
    import pyarrow.parquet as pq

    table = pq.read_table(path)
    missing = PARQUET_REQUIRED_COLUMNS.difference(table.column_names)
    if missing:
        raise ValueError(f"parquet manifest missing columns: {sorted(missing)}")

    rows = table.to_pylist()
    for row in rows:
        for bytes_key, sha_key, size_key in PARQUET_SENSOR_VALIDATION:
            payload = row[bytes_key]
            if len(payload) != row[size_key]:
                raise ValueError("size mismatch")
            if hashlib.sha256(payload).hexdigest() != row[sha_key]:
                raise ValueError("sha mismatch")
    return rows
```

PyTorch dataset boundary:

```python
class NuScenesParquetManifestDataset(Dataset):
    def __init__(self, manifest_path: str | Path) -> None:
        self.rows = load_parquet_manifest(manifest_path)

    def __getitem__(self, index: int) -> dict[str, Any]:
        return dict(self.rows[index])
```

Final comparison:

- Use Go when the platform needs small deployable tools, strict file contracts, byte provenance, and fast deterministic tests.
- Use Python/Spark when the platform needs distributed relational transformations, notebooks, ML libraries, and team familiarity.
- The clean boundary is the point: Go produces trusted Parquet contracts; Python consumes them for analytics and training.

## Refactors Applied for the Demo

The Go workflow was lightly refactored to make these teaching points easier to show:

- `requireArtifacts(paths ...string)` centralizes upstream artifact checks while preserving error wording.
- `eventFields`, `rowEventSpec[T]`, and `publishRows[T]` remove repeated event loops without hiding event topics or keys.
- `parquetManifestSensorSpec` makes the seven required sensor projections explicit while keeping the denormalized `ParquetManifestRow` schema stable.

The public behavior remains unchanged: existing Makefile commands, event topics, Parquet paths, manifest schemas, and Python loader contracts stay the same.
