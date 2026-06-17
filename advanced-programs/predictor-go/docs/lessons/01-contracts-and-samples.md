# 01 Contracts And Samples

Start with the sample files, not live sockets.

Run:

```bash
go run ./cmd/predictorctl validate-samples
```

This lesson maps `basics/10-Struct` to real data contracts:

- `RawPayload`
- `NormalizedOrderBook`
- `InstrumentSpec`

The sample files contain a UTF-8 BOM, so loading uses an explicit BOM-safe
reader. The validator also records fixture asymmetries: DEX rows are skipped,
one raw MEXC spot row is only a subscription acknowledgment, and raw/normalized
CEX coverage is not one-to-one.

Exercise: add a failing test for an invalid `bids_json` value, then fix the
fixture or validation code.
