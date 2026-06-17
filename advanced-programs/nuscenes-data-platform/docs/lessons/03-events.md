# 03 Kafka-Shaped Events

The first event bus is local and deterministic.

Run:

```bash
make ingest-demo
head runs/events/events.jsonl
```

The mock writes event envelopes with topic, key, dataset version, schema version, sample ID, scene ID, source path, and payload. It has the same producer boundary a Redpanda or Kafka adapter would implement later.

Go concepts:

- interfaces from `basics/11-Interfaces`
- producer contracts from `advanced-programs/predictor-go`

Exercise: add a new topic for annotation events without changing the Parquet writer.
