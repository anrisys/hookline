# Hookline — Backlog

End-to-end slices in order. Each one runs through `docker compose up` and can be demoed on its own. References are to [requirements.md](requirements.md).

## S0 — Walking skeleton
Go module with `api|worker|chaos` modes, goose migrations, `/healthz`, SvelteKit skeleton, multi-stage Dockerfiles, compose with Postgres, CI (lint, test, build).
**Done when:** `docker compose up` starts every service healthy, the dashboard shows an "API: ok" tile fetched from `/healthz`, and CI is green on a PR.

## S1 — Endpoints CRUD (R5–R7, R18, R19)
Bearer auth, endpoint API with a generated secret, SvelteKit endpoints page (SSR load + form actions).
**Done when:** you create an endpoint in the UI and see its secret once, it appears in the list after a reload, deleting it removes it, and `curl` without the key gets `401`.

## S2 — Ingest + single delivery (R1–R3, R9, R11, R16)
`POST /events` with idempotency and fan-out; worker claims with SKIP LOCKED and sends a signed POST; attempts are recorded; chaos receiver at `fail=0`.
**Done when:** `curl POST /events` twice with the same key creates one event (a different payload gets `409`), the chaos receiver logs a request whose signature checks out, and `psql` shows a `succeeded` delivery with one attempt row.

## S3 — Retries, dead, replay (R8, R10, R12, R13)
Backoff with jitter, dead state, replay endpoint, lease reaper.
**Done when:** with the endpoint at `/chaos?fail=1` and a short retry schedule, the delivery shows N attempts with growing gaps and ends `dead`. Switching to `fail=0` and calling replay gives a new delivery that succeeds with `replay_of` set. Killing the worker with `kill -9` mid-claim leads to the row being reclaimed after the lease expires.

## S4 — Events UI + test event + replay button (R4, R20)
Events list (paged, filtered), detail timeline, send-test-event form, replay button.
**Done when:** you send a test event from the UI, open its detail page, see the attempt timeline with status, latency and snippet, and press replay on a dead delivery to see a new delivery appear.

## S5 — Live updates (R17, R21)
LISTEN/NOTIFY in the worker and API, `/stream` SSE, SvelteKit proxy, runes-based store.
**Done when:** with the events page open and nothing reloaded, a failing delivery's status and attempt count update live through each retry until `dead`. Restarting the API, the page reconnects and catches up.

## S6 — Graceful shutdown (R14, R15)
Signal handling, release of claimed rows, in-flight drain with a deadline.
**Done when:** with deliveries running against `/chaos?delay=5s`, `docker compose stop worker` lets those calls finish (their attempts are recorded as succeeded), claimed-but-unstarted rows go back to `pending`, and an integration test proves both.

## S7 — Tracing end to end (R22, N1, N3)
OTel SDK in Go and SvelteKit, collector, Tempo, traceparent stored on the delivery row, `queue.wait` and attempt spans, chaos server spans, slog with `trace_id`.
**Done when:** sending a test event from the UI produces one trace in Grafana/Tempo spanning SvelteKit → API → queue.wait → attempt(s) → chaos, including retries, and a log line's `trace_id` jumps to that trace.

## S8 — Metrics, dashboard, alert (N2, N4)
Prometheus metrics, provisioned Grafana dashboard, queue-lag alert rule.
**Done when:** the dashboard shows success rate, p50/p99 attempt latency by endpoint, queue depth and lag. Pausing the worker makes lag climb past 30s and the alert fires; resuming clears it.

## S9 — Per-endpoint timeout: the incident fix (R5 `timeout_ms`)
Attempts honour `timeout_ms`; editable in the UI.
**Done when:** this incident story runs from the README script. Endpoint A points at `/chaos?delay=20s`, B at a healthy URL. Lag climbs and the alert fires. Tempo shows A's slow attempts. Setting A's `timeout_ms` to 2000 brings lag down while B stays healthy.

## S10 — Load test + README (N6)
k6 script, results captured, README with diagram, the Postgres-as-queue section, incident story, limitations, 100× notes.
**Done when:** `k6 run loadtest/ingest.js` reproduces the published events/sec and p99 attempt latency within ±20% on the same machine, and a reader can follow the README from clone to incident demo.

## Stretch
- **S11** Per-endpoint concurrency cap. Done when a slow endpoint can't take up more than K workers, shown with the S9 scenario without the timeout fix.
- **S12** Deploy to Fly.io or a VPS. Done when a public URL serves the dashboard with the alert routed somewhere.
