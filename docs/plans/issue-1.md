# Plan — Issue #1: Go module skeleton

Issue: https://github.com/anrisys/hookline/issues/1 · Milestone: S0 · Branch: `issue-1-go-skeleton`

Sources: `architecture.md` (Components, Code layout, Configuration), `requirements.md` (R15, R18, N3, N5), `backlog.md` (S0).

## A. Acceptance criteria
- **AC1** `hookline api` with a reachable `DATABASE_URL` applies embedded goose migrations; `goose_db_version` shows version 1.
- **AC2** `GET /healthz` returns 200 when the DB answers a ping, 503 when it doesn't. No auth.
- **AC3** Migrations run under an advisory lock; two API processes starting at once don't race.
- **AC4** Config comes from env. A missing required variable stops startup with a clear error and a non-zero exit.
- **AC5** `hookline worker` and `hookline chaos` start, and all modes exit 0 on SIGTERM within `SHUTDOWN_TIMEOUT`.
- **AC6** An unknown mode prints usage and exits 2.

## B. Plan
| Area | Files |
|---|---|
| Backend | `cmd/hookline/main.go`, `internal/config/`, `internal/telemetry/log.go`, `internal/api/`, `internal/worker/`, `internal/chaos/` |
| Database | `internal/store/db.go`, `internal/store/migrate.go`, `internal/store/migrations/00001_baseline.sql`, `internal/store/storetest/` (temp-DB helper) |
| Frontend | Not touched (#2) |
| Infra | `.gitignore`, `.env.example`, `Makefile`, `.golangci.yml`. No Dockerfile/compose (#3). |

sqlc is not added yet (no queries until #5).

## Implementation notes (2026-11-07)
- **Ownership changed:** for S0 the AI wrote everything below, including config, healthz, run/shutdown, `store.Migrate` and all tests (user decision; later issues follow the split in `CLAUDE.md`). The table in C is kept for reference.
- **Shared plumbing:** health handler and graceful `Serve` live in `internal/httpx` (used by all three modes) instead of `api.Health`. Test helpers are in `internal/httpx/httpxtest` and `internal/store/storetest`.
- **Added `LOG_LEVEL`** (default `info`, validated).
- **Lock test rewritten.** Two concurrent `Migrate` calls pass even with the session locker removed: goose's `Up` first runs an unlocked `HasPending` that creates the version table with retries, which already makes that race wait. `TestMigrateWaitsForAdvisoryLock` holds goose's advisory lock from another session and asserts that `Migrate` blocks and applies nothing. Verified it fails with the locker removed.
- **Requests don't inherit the shutdown context**, so a SIGTERM lets in-flight requests finish (tested).
- **goose retries the lock every 5s for up to 5 min**, so a second API replica can wait that long at startup.
- **Lint:** `noctx` is off for `_test.go`; the production code uses `net.ListenConfig` with the run context.

## C. Ownership
| Piece | Owner | Notes |
|---|---|---|
| go.mod, Makefile, lint config, gitignore, `.env.example`, embed + baseline SQL, slog setup, `main.go` mode switch, temp-DB test helper | AI scaffolds | |
| `config.Load(mode, getenv)` | **I write** | Inject `getenv`; per-mode required vars (`DATABASE_URL` for api/worker); `time.ParseDuration`; report all errors via `errors.Join`. |
| `store.Migrate(ctx, db, fsys)` | **I write** | goose v3 `NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(lock.NewPostgresSessionLocker()))`; `*sql.DB` via `pgx/v5/stdlib`. |
| `/healthz` handler | **I write** | Depends on a `Pinger` interface; 1s ping timeout from request ctx; 200 `{"status":"ok"}` / 503 `{"status":"unavailable"}`; no caching. |
| `Run(ctx, cfg) error` per mode | **I write** | On ctx cancel: `srv.Shutdown` with `SHUTDOWN_TIMEOUT`; `http.ErrServerClosed` is success; pre-ready errors return non-nil. |

## D. Tests (write first)
**config (unit):** non-default values all read; defaults `API_ADDR=:8080`, `WORKER_ADDR=:8081`, `CHAOS_ADDR=:8090`, `SHUTDOWN_TIMEOUT=30s`; missing `DATABASE_URL` errors for api/worker, ok for chaos; `SHUTDOWN_TIMEOUT` = `abc`/`0s`/`-1s` rejected; empty = unset; multiple errors all reported.

**healthz (unit, fake pinger):** ok → 200 JSON; error → 503; hanging ping → 503 within ~1s; fail→ok→fail sequence (no caching); no `Authorization` → 200; `POST` → 405.

**store (integration, `TEST_DATABASE_URL`):** fresh DB → version 1; migrate twice → still one row per version; two concurrent migrations with a test-only `CREATE TABLE t` + `pg_sleep(0.5)` migration → both succeed, table and version row exist once; broken migration → error, version unchanged; unreachable URL fails within a bound; healthz with real pool → 200, closed pool → 503.

**lifecycle:** `api.Run`/`chaos.Run`/`worker.Run` return nil after ctx cancel once `/healthz` is 200; in-flight 300ms request completes during shutdown; port in use → error. Built-binary test: start `worker` and `chaos`, wait for readiness, SIGTERM → exit 0; unknown mode → exit 2.

| AC | Check |
|---|---|
| AC1 | migrate test + manual `goose_db_version` query |
| AC2 | healthz unit + integration tests + manual curl with DB up/down |
| AC3 | concurrent migrate test |
| AC4 | config tests + manual run without `DATABASE_URL` |
| AC5 | lifecycle tests + built-binary SIGTERM test + manual |
| AC6 | built-binary test |

## E. Commits
1. `chore: init Go module, Makefile, lint config, gitignore`
2. `feat(config): load and validate env config per mode`
3. `feat(telemetry): JSON slog logger`
4. `feat(store): pgx pool and embedded goose migrations with advisory lock`
5. `feat(api): HTTP server with DB-backed /healthz`
6. `feat(worker,chaos): worker and chaos server stubs with /healthz`
7. `feat(cmd): hookline mode switch with signal-driven graceful shutdown`

PR body includes `Closes #1`.

## F. Verify
Local values live in `.env` (gitignored, copied from `.env.example`). The Makefile loads it; nothing is exported into your shell permanently. For an ad-hoc terminal session: `set -a; source .env; set +a`.

```bash
cp .env.example .env
make db-up                       # postgres:17 container "hookline-pg" on :5432
make fmt-check vet lint
make test                        # go test -race -count=1 ./...
make build                       # bin/hookline

make run-api &                   # then:
curl -i localhost:8080/healthz   # 200
make db-psql Q='select version_id from goose_db_version'
docker stop hookline-pg && curl -i localhost:8080/healthz   # 503
docker start hookline-pg
kill -TERM %1; wait %1; echo "exit=$?"                      # 0

set -a; source .env; set +a
./bin/hookline worker & P=$!; sleep 1; kill -TERM $P; wait $P; echo "exit=$?"   # 0
./bin/hookline chaos  & P=$!; sleep 1; kill -TERM $P; wait $P; echo "exit=$?"   # 0
env -u DATABASE_URL ./bin/hookline api; echo "exit=$?"                          # non-zero
./bin/hookline nope; echo "exit=$?"                                             # 2
```
No `docker compose` check: the stack isn't touched.

## G. Decisions (resolved with recommended defaults)
1. Integration tests use `TEST_DATABASE_URL`; skip when unset locally, fail when `CI=true`.
2. Worker serves `WORKER_ADDR=:8081` with `/healthz` now (`/metrics` joins in #21).
3. API fails fast if the DB is unreachable at startup; compose `depends_on: service_healthy` handles ordering (#3).
4. Worker schema-version check deferred to #9.
5. Stdlib `switch os.Args[1]`, no CLI library.

Assumptions: `go 1.27` in go.mod; `API_KEY` not required until #5; worker pings the DB once at startup; chaos has no DB.

## H. How tests could pass wrongly
- Integration tests skipped when `TEST_DATABASE_URL` is unset → CI fails on skip; check `-v` locally.
- A trivial migration never races → the lock test uses a slow, non-idempotent migration.
- Fake pinger only → real-pool integration test and manual `docker stop` check.
- Cached health → fail→ok→fail sequence.
- Ctx-cancel tests don't prove signal wiring → built-binary SIGTERM test, after readiness.
- A mode that exits immediately also exits 0 → require `/healthz` 200 before SIGTERM.
- Test values equal to defaults → normal case uses non-defaults.
- "No error" migrate → assert `goose_db_version` rows.
