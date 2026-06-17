# 03 Concurrency And Backpressure

The writer service uses `context.Context`, a bounded channel, and a
`sync.WaitGroup`.

That connects directly to `basics/13-Concurrency`: one goroutine reads Redis,
another goroutine writes batches, and cancellation stops both.

Run:

```bash
go test ./internal/app ./internal/storage
```

Exercise: lower the writer channel size and explain what happens when Redis
produces faster than the sink can write.
