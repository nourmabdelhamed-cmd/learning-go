# 06 Tests, CI, And Handoff

The full local demo uses real data, but CI must stay small and deterministic.

Run:

```bash
make test
make ci-demo
```

The committed fixture is intentionally tiny and fake. It exists only to verify contracts in GitHub Actions.

Engineering concepts:

- generated artifacts stay out of Git
- real archives stay local
- CI verifies workflow shape without expensive data movement

Exercise: break one required camera path in the fixture and confirm the Go validation fails.
