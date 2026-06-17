# 06 Tests And Handoff

The fast test contract is:

```bash
go test ./...
```

It must not need Redis, ClickHouse, network access, or live exchanges. External
integration checks should stay opt-in.

The README documents setup, run, test, Compose, and state boundaries so another
learner can rerun the project without reading implementation internals first.

Exercise: add a GitHub Actions job that runs only the fast test contract.
