# Hookline — Requirements

Hookline is a webhook delivery service. It accepts events and delivers them to subscribed endpoints, with signing, retries, a replay option, and observability throughout. Source brief: [assessment.md](assessment.md).

## Scope

**In:** Go API, worker and chaos receiver in one binary; Postgres as the only datastore; SvelteKit dashboard; OTel tracing, Prometheus metrics, structured logs; docker compose stack; CI; README with load test and incident story.
**Out (v1):** multi-tenancy, user login, ordering guarantees, idempotency-key expiry, `Retry-After`, SSRF protection, secret encryption at rest, cloud deploy (optional final item).

## Functional requirements

### Events
- **R1** `POST /events` takes `{type: string, payload: JSON}`, up to 256 KB. The server assigns a UUIDv7 `id`.
- **R2** The `Idempotency-Key` header is required (missing → `400`). If a key repeats with the same payload hash → `200` with the original event. Same key with a different payload → `409`. Keys never expire.
- **R3** The event and its deliveries are inserted in one transaction, one delivery per active endpoint subscribed to `type` or `*`. An event with no matching endpoint is allowed (zero deliveries).
- **R4** `GET /events` is paged by event ID, 50 per page, filterable by `type` and delivery status. `GET /events/{id}` returns its deliveries and every attempt.

### Endpoints
- **R5** CRUD on `/endpoints`. Fields: `url`, `event_types[]`, `timeout_ms` (default 10000), `enabled`.
- **R6** The server generates the secret (`whsec_…`). It is returned only on create and on `POST /endpoints/{id}/rotate-secret`.
- **R7** Delete is a soft delete. The endpoint's pending deliveries become `cancelled`, and history is kept.

### Delivery
- **R8** States: `pending → in_flight → succeeded | pending (retry) | dead`, plus `cancelled`.
- **R9** Each request is `POST url` with headers `Hookline-Id` (delivery ID), `Hookline-Timestamp` (unix seconds), `Hookline-Signature: v1=<hex(HMAC-SHA256(secret, "{ts}.{body}"))>`, and `traceparent`. Redirects are not followed.
- **R10** Success means a 2xx response. Any other status, a timeout or a connection error is retried with full-jitter exponential backoff: base 2s, ×2 each time, cap 10 min, 8 attempts in total (all set by env vars). Then the delivery becomes `dead`.
- **R11** Each attempt records: number, start time, latency, status code or error, and the first 2 KB of the response body (with invalid UTF-8 cleaned up).
- **R12** `POST /deliveries/{id}/replay` (for `dead` or `succeeded` deliveries only) creates a new delivery with `replay_of = id` and a fresh attempt count.
- **R13** Delivery is at-least-once with no ordering guarantee. Receivers deduplicate on `Hookline-Id`.

### Workers and shutdown
- **R14** Workers claim batches with `FOR UPDATE SKIP LOCKED` in a short transaction and take a lease. A reaper sets expired leases back to `pending`.
- **R15** On SIGTERM: stop claiming, set claimed-but-not-started deliveries back to `pending`, and let in-flight calls finish within a 30s shutdown deadline.

### Chaos receiver
- **R16** `hookline chaos` serves `/chaos?fail=<0..1>&delay=<dur>&jitter=<dur>&status=<code>`, a URL-configured receiver for demos and load tests. It runs as its own compose service.

### Live updates and auth
- **R17** `GET /stream` (SSE) sends `{delivery_id, event_id, endpoint_id, status, attempt}` on every state change, in every API instance (via LISTEN/NOTIFY). After a reconnect the client refetches; there is no `Last-Event-ID` replay.
- **R18** Every route except `/healthz`, `/metrics` and `/chaos` requires `Authorization: Bearer <API_KEY>`. The dashboard has no login, and the key stays on the SvelteKit server.

### Dashboard (SvelteKit, Svelte 5)
- **R19** Endpoints page: SSR `load`, form actions for create and delete.
- **R20** Events list and an event detail page showing each delivery's attempts as a timeline. A "send test event" form and a replay button.
- **R21** Live status through SSE using `$state` and `$effect`, proxied through a SvelteKit route.
- **R22** A server hook starts or continues the trace and forwards `traceparent` on every backend call.

## Non-functional requirements
- **N1 Tracing:** one trace covers SvelteKit → API → `queue.wait` → each attempt (→ chaos receiver). Trace context is stored on the delivery row. A replay starts a new trace with a span link to the original.
- **N2 Metrics:** `attempts_total{endpoint_id,outcome}`, `attempt_duration_seconds{endpoint_id}` histogram, `deliveries_terminal_total{state}`, `queue_depth`, `queue_oldest_ready_age_seconds`. The two queue gauges count ready rows only and are published by the worker alone.
- **N3 Logs:** JSON slog with `trace_id` and `span_id` on every line inside a request or job.
- **N4 Ops:** `docker compose up` starts the app, Postgres, OTel collector, Tempo, Prometheus and Grafana, with one provisioned dashboard and an alert for queue lag over 30s for 1 minute.
- **N5 CI:** lint (golangci-lint, eslint, svelte-check), tests (Go integration tests against a Postgres service), image build without push.
- **N6 Docs:** README with an architecture diagram, why Postgres works as the queue and where it stops scaling, k6 results (events/sec, p99 attempt latency), the incident story, limitations, and what changes at 100× scale.
