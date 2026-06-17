# 05 Observability And Compose

Long-running commands expose:

- `/healthz`
- `/metrics`

This follows `basics/19-Webserver` and `advanced-programs/PrometheusHTTPServer`.
Compose provides Redis and ClickHouse boundaries similar to the CRUD examples.

Run:

```bash
docker compose up --build redis clickhouse predictor-writer
curl http://localhost:8088/healthz
curl http://localhost:8088/metrics
```

Exercise: add a counter for skipped DEX rows in sample replay.
