## Who writes what
- AI writes: scaffolding, config, Docker/CI, health/shutdown plumbing, and
  their tests. Explain anything non-obvious.
- I write: schema/migration SQL, claim query, retry/backoff, signing,
  idempotency, tracing, and their tests. For these, give hints and
  behaviors to test, not code. Review my tests and code when asked.