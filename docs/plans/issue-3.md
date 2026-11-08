# Plan — Issue #3: Multi-stage Dockerfiles + docker compose with Postgres

Issue: https://github.com/anrisys/hookline/issues/3 · Milestone: S0 · Branch: `issue-3-docker-compose`

Sources: `backlog.md` (S0), `requirements.md` (Scope, R16, R18, N4, N5), `architecture.md` (Components, Code layout `deploy/`, Configuration), `plans/issue-1.md` and `plans/issue-2.md` (implementation notes).

## A. Acceptance criteria
- **AC1** Multi-stage Go Dockerfile, `distroless/static` final image; one image runs api, worker and chaos.
- **AC2** Multi-stage SvelteKit (adapter-node) Dockerfile; the final image holds `build/` only, no dev deps.
- **AC3** `deploy/docker-compose.yml`: postgres, api, worker, chaos, web, each with a healthcheck.
- **AC4** `docker compose up` brings every service to healthy and the dashboard shows "API: ok".

One session. CI image builds are #4.

## B. Plan
| Area | Change |
|---|---|
| Backend | `cmd/hookline/main.go`: `hookline healthcheck <url>` (GET, exit 0 on 200, 1 otherwise, 2 on bad usage; 2s timeout; no redirects; no config). Distroless has no shell or curl. |
| Database | No schema change. `postgres:17`, named volume `pgdata`, `pg_isready` healthcheck. |
| Frontend | `web/src/routes/healthz/+server.ts`: 200, no API call. |
| Infra | `Dockerfile` + `.dockerignore` (root, Go). `web/Dockerfile` + `web/.dockerignore`. `deploy/docker-compose.yml`. Root `compose.yaml` that `include`s it. Makefile `up`, `down`, `smoke`. |

Design:
- Go: `golang:1.27` builder, `CGO_ENABLED=0 -trimpath -ldflags="-s -w"`, BuildKit cache mounts; final `gcr.io/distroless/static-debian12:nonroot`, `ENTRYPOINT ["/hookline"]`, `command` per service.
- Web: `node:24-slim` stages deps → build → runtime (`build/` + `package.json`, user `node`).
- `stop_grace_period: 40s` on api/worker so docker's 10s SIGKILL doesn't cut `SHUTDOWN_TIMEOUT=30s`.
- depends_on: api → postgres healthy; worker → postgres + api healthy (migrations run in api); web → api healthy.
- Compose reads the root `.env` for `${VAR}` interpolation only. Its `DATABASE_URL` points at localhost, so the compose file sets container URLs explicitly and never interpolates `${DATABASE_URL}`.
- `.dockerignore` keeps `.git`, `bin/`, `.env`, `web/.env`, `node_modules`, build output out of contexts.

## Implementation notes (2026-11-08)
- **Measured:** `hookline:local` is 23.2MB, user `nonroot:nonroot`. `hookline-web:local` is 333MB, almost all of it the `node:24-slim` base (uncompressed); the app layer is `build/` + `package.json`. A cold `make up` reaches all-healthy in about 14s.
- **The web runtime really needs nothing but `build/`.** Checked by running `node build` from a copy with no `node_modules` and no `src/`. The `#lib`/`#server` hits seen when grepping the build output are JSDoc `@import` comments, not real imports.
- **Healthcheck:** exit 0 only on 200. Redirects aren't followed. 2s timeout, longer than the API's 1s DB ping, so an outage shows as a 503 and not as a client timeout. It runs before `config.Load`, so it needs no env.
- **Unhealthy doesn't mean restarted.** Plain Docker (outside Swarm) doesn't restart unhealthy containers. With Postgres stopped, api and worker show `unhealthy`, web stays healthy and shows "API: down", and all of them recover when Postgres is back. That's the behaviour we want.
- **Shutdown:** `docker compose stop` takes about 1.4s; every container exits 0, and api, worker and chaos log "stopped".
- **Restart:** `down` + `up` keeps `pgdata`; goose applies nothing new.
- **`hookline:local` builds three times** (api, worker and chaos share one YAML anchor). All three are cache hits on one tag, so it costs almost nothing. Giving `build:` to api alone would mean relying on Compose's ordering for worker and chaos on a fresh machine.
- **Choices beyond the plan:**
  - Every host port binds to `127.0.0.1` (dev credentials).
  - web gets `ORIGIN=http://localhost:3000` now, because adapter-node's form-action origin check needs it in S1.
  - chaos gets an explicit empty `DATABASE_URL`.
  - `start_interval: 1s` speeds up the first healthy result.
  - The root `compose.yaml` sets `name: hookline`.
- **YAML gotcha:** the web healthcheck's JS contains ` ? 0 : 1`, and the `: ` made YAML parse it as a mapping, so the string is quoted.
- **The Makefile exports `.env`** into recipes, so `make up` passes `DATABASE_URL=localhost…` into Compose's environment. That's harmless because the compose file never interpolates it.
- **Verified:** `make fmt-check vet lint test`, `make web-check web-lint web-test`, `make smoke`, plus the failure, shutdown and restart checks in F.

## C. Ownership
All **AI scaffolds** (Docker, compose, health plumbing and tests). The user reviews.

## D. Tests
**`hookline healthcheck`:** 200 → 0 · 503, 500, 404 → 1 · connection refused → 1 · slow server → 1 within timeout · 3xx not followed → 1 · no arg / two args → 2 · malformed URL → non-zero, no panic · runs with empty env.

**Web `/healthz`:** 200; never calls the API client.

| AC | Check |
|---|---|
| AC1 | `docker image inspect` user `65532`, small size; three modes from one image |
| AC2 | runtime image has no `src/` or `node_modules`; `node build` starts |
| AC3, AC4 | `docker compose up -d --wait` exits 0; `curl localhost:3000` shows "API: ok" |
| Restart | `down` + `up` keeps the volume, no pending migrations |
| Failure | stop postgres → api unhealthy, page "API: down"; start → back to ok |
| Shutdown | `docker compose stop` → Go containers exit 0 with "stopped" |

## E. Commits
1. `docs(plans): plan for issue #3 docker and compose`
2. `feat(cmd): healthcheck subcommand for distroless probes`
3. `feat(web): /healthz liveness route`
4. `build: multi-stage Go Dockerfile and .dockerignore`
5. `build(web): multi-stage SvelteKit Dockerfile and .dockerignore`
6. `chore(deploy): compose stack with postgres, api, worker, chaos, web`
7. `chore: Makefile up/down/smoke targets`
8. `docs(plans): issue #3 implementation notes`

## F. Verify
```bash
make fmt-check vet lint test
make web-check web-lint web-test
make db-down 2>/dev/null
docker compose build
docker compose up -d --wait
curl -s localhost:3000 | grep -o 'API: [a-z]*'      # API: ok
docker compose stop postgres; sleep 3; curl -s localhost:3000 | grep -o 'API: [a-z]*'  # API: down
docker compose start postgres
docker compose stop && docker compose ps -a         # exit 0
docker compose down && docker compose up -d --wait
docker compose down -v
```

## G. Decisions (all defaults accepted)
- **D1** Root `compose.yaml` with `include: [deploy/docker-compose.yml]` (Compose v5.6.1 here).
- **D2** `hookline healthcheck` subcommand.
- **D3** Web probe hits a new `/healthz` route, independent of the API.
- **D4** Host ports: web 3000, api 8080, chaos 8090, postgres `127.0.0.1:${POSTGRES_PORT:-5432}` (`make db-down` first if `hookline-pg` runs).
- **D5** Images pinned by tag, not digest.
- **D6** Web runtime on `node:24-slim`.
- **D7** `API_KEY` wiring deferred to S1.

## H. Ways the checks could pass against a wrong implementation
- Unit tests pass but the compose probe uses the wrong port or host → `up --wait` catches it; probes use `127.0.0.1`.
- All healthy but the page says "API: down" (wrong `API_URL`) → smoke greps "API: ok".
- CGO binary fails only in distroless → only container start catches it.
- Worker healthy before tables exist → depends_on api healthy; no test until S2.
- Stop "clean" but actually SIGKILLed → check exit code 0 and the "stopped" log, not 137.
- `.env` copied into an image → inspect the build stage.
- `node build` works only because dev deps were copied → runtime image contents check.
