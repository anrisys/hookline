# Hookline — Architecture

## Components

```
 Browser ──► SvelteKit (adapter-node) ──Bearer + traceparent──► hookline api ──┐
   ▲  SSE        │  server hook: OTel span, forwards traceparent               │
   └─────────────┘  /stream proxy route                                        ▼
                                                                          Postgres
 hookline worker ── claim (SKIP LOCKED) / record attempt / NOTIFY ──────────►  ▲
        │                                                                     │
        └── signed POST + traceparent ──► receivers (hookline chaos, real URLs)

 api, worker, chaos, SvelteKit ──OTLP──► OTel collector ──► Tempo
 api, worker ──/metrics──► Prometheus ──► Grafana (dashboard + queue-lag alert)
```

| Component | Responsibility |
|---|---|
| `hookline api` | REST and auth, ingest with idempotency and fan-out, replay, `/stream` SSE (LISTENs on `delivery_events`), runs migrations at startup under an advisory lock |
| `hookline worker` | Worker pool: claim → sign → POST → record attempt → schedule the retry or finish; lease reaper; queue gauges |
| `hookline chaos` | Receiver whose failures and latency are set by URL query; emits server spans |
| SvelteKit | SSR pages, form actions, SSE proxy; holds the API key; root span for each request |
| Postgres | Events, endpoints, deliveries (the queue), attempts; LISTEN/NOTIFY |

Code layout: `cmd/hookline` (mode switch), `internal/{api,worker,chaos,store(sqlc),signing,telemetry}`, `web/` (SvelteKit), `deploy/` (compose, Grafana/Prometheus/collector config), `loadtest/`.

## Data model (core columns)

- `endpoints(id, url, secret, event_types text[], timeout_ms, enabled, deleted_at)`
- `events(id uuidv7, type, payload jsonb, idempotency_key UNIQUE, payload_hash, created_at)`
- `deliveries(id, event_id, endpoint_id, status, attempt_count, next_attempt_at, lease_expires_at, claimed_by, traceparent, replay_of, created_at)`
  index `(next_attempt_at) WHERE status='pending'`
- `attempts(id, delivery_id, n, started_at, latency_ms, status_code, error, response_snippet)`

## Data flow

1. **Ingest:** SvelteKit → `POST /events`. In one transaction: `INSERT … ON CONFLICT (idempotency_key) DO NOTHING`. On conflict, compare `payload_hash` and return `200` or `409`. Otherwise insert one delivery for each matching endpoint with `next_attempt_at = now()` and the current `traceparent`.
2. **Claim:** the worker runs `UPDATE deliveries SET status='in_flight', lease_expires_at=now()+lease, claimed_by=$w WHERE id IN (SELECT id … WHERE status='pending' AND next_attempt_at<=now() ORDER BY next_attempt_at LIMIT $n FOR UPDATE SKIP LOCKED) RETURNING …` and commits right away.
3. **Deliver:** extract the stored `traceparent` → `queue.wait` span (ready time → claim time) → `delivery.attempt` client span → signed POST with the endpoint's `timeout_ms`.
4. **Record:** in one transaction, insert the attempt, update the delivery (succeeded, or pending with a backoff, or dead), and run `pg_notify('delivery_events', …)`.
5. **Stream:** the API's single listener connection fans the notification out to its SSE clients. SvelteKit proxies the stream; the page updates its `$state`.

## Key decisions

**D1 Postgres as the queue (given).** One datastore with transactional enqueue: the event and its deliveries are committed atomically, so no outbox is needed. Where it stops scaling: claim contention and index churn at roughly a few thousand deliveries/sec, vacuum pressure on a table with constant updates, and one primary for writes. The README covers what changes at 100×: partitioning `deliveries`, a separate attempts store, or moving to Kafka/SQS with an outbox.

**D2 Lease-based claim, not lock-held-during-HTTP.**
| | Hold the row lock through the HTTP call | **Short claim transaction + lease (chosen)** |
|---|---|---|
| DB connections | One per in-flight delivery (timeouts up to 10s+) | Held only briefly while claiming and recording |
| Crash recovery | Automatic when the connection drops | Reaper resets expired leases (lease = timeout + margin) |
| Duplicate-send risk | Low | A worker stalled past its lease may resend; acceptable under at-least-once |

Graceful shutdown: claimed rows that haven't started go back to `status='pending'`; in-flight calls finish within the deadline.

**D3 SSE across processes through LISTEN/NOTIFY.** The worker changes state; the API streams it. NOTIFY is part of the same transaction as the state change, so clients never see a change that wasn't committed, and nothing else needs to be deployed. Alternative: the API polls `deliveries.updated_at` (simpler, but adds latency and database load). Notifications are lost while a client is disconnected, so clients refetch on reconnect.

**D4 Trace continuity through the delivery row.** `traceparent` is stored at ingest. Every attempt, including retries hours later, is a child of the ingest trace, so one trace shows the whole story. `queue.wait` makes queue time visible, and forwarding `traceparent` to receivers puts the slow chaos span inside the same trace. A replay starts a new root trace with a span link, so traces don't grow without limit.

**D5 Lag measures ready work only.** `queue_oldest_ready_age = now() - min(next_attempt_at) WHERE status='pending' AND next_attempt_at<=now()`. Measuring from `created_at` would count rows waiting out a backoff, and the alert would fire on normal retries. Only the worker publishes the gauge, which avoids duplicate series.

**D6 Chaos receiver is its own mode and service.** It ships in the same binary, but running it separately means load tests measure Hookline itself, not Hookline competing with its own receiver for CPU.

## Configuration (env)
`DATABASE_URL, API_KEY, WORKER_CONCURRENCY, CLAIM_BATCH, LEASE_SECONDS, RETRY_MAX_ATTEMPTS, RETRY_BASE, RETRY_CAP, SHUTDOWN_TIMEOUT, OTEL_EXPORTER_OTLP_ENDPOINT`
