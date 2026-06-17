# 02 Interfaces And Storage

The storage package uses small interfaces before concrete systems:

```go
type Sink interface {
    WriteRaw(context.Context, []contract.RawPayload) error
    WriteNormalized(context.Context, []contract.NormalizedOrderBook) error
    Close() error
}
```

This follows `basics/11-Interfaces`: tests use `MemorySink`, local runs can use
Parquet, and deployed runs can use ClickHouse. The rest of the app does not need
to know which sink is active.

Run:

```bash
go test ./internal/storage
```

Exercise: add a new sink that writes JSONL files without changing the writer
service.
