# 04 Websocket Adapters

Live exchange collectors are intentionally staged after sample replay. The
`internal/exchange` package starts with a tested `LocalBook` and a minimal
websocket adapter boundary.

This reuses the read/write pump thinking from `advanced-programs/WebsocketsChat`
without forcing learners to debug seven exchange APIs before they understand the
contract.

Run:

```bash
go test ./internal/exchange
go run ./cmd/predictor-exchange -exchange Binance
```

Exercise: add a parser test for one Binance raw sample payload before opening a
real websocket.
