Project: “Hookline” (name it whatever you like)

Architecture: one Go module, one binary with api and worker modes, Postgres as the only datastore, and SvelteKit as the frontend. Skip Kafka and Redis. Using Postgres as the queue (SELECT ... FOR UPDATE SKIP LOCKED).

Backend requirements (Go)

POST /events accepts an event, with an Idempotency-Key header. A duplicate key must not create a second event.

CRUD for endpoints (URL, secret, subscribed event types). Auth can be a single static API key.

A worker pool claims pending deliveries via SKIP LOCKED.

Each delivery is an HTTP POST signed with HMAC-SHA256 and a timestamp header.

Retries use exponential backoff with jitter, up to N attempts, then the delivery goes to dead. Add a manual “replay” endpoint.

Each attempt is stored with status code, latency, and response snippet.

Graceful shutdown: in-flight deliveries finish and unclaimed work is released.

A built-in “chaos receiver” endpoint that fails randomly or slowly, for demos.

GET /stream (SSE) pushes delivery status changes to the dashboard.

Frontend requirements (SvelteKit, Svelte 5)

Endpoints page, using load functions for SSR and form actions for create and delete.

Events list with a detail view showing all delivery attempts as a timeline.

Live updates via SSE using runes ($state,$effect).

A “send test event” form and a “replay” button.

A server hook that forwards the traceparent header on every backend call.

Observability requirements

Tracing (OpenTelemetry): one trace from the browser-facing SvelteKit request through the API, the queue wait, and each delivery attempt. Store the trace context in the delivery row so the worker continues the same trace. This is the part most candidates skip.

Metrics (Prometheus): delivery success rate, attempt latency histogram, queue depth, and oldest-pending-job age (queue lag).

Logs: structured slog with trace_id on every line.

Stack: docker compose with Postgres, the app, an OTel collector, Tempo or Jaeger, Prometheus, and Grafana. Include one provisioned dashboard and one alert rule (for example, queue lag above 30s).

DevOps requirements

Multi-stage Dockerfiles for Go and SvelteKit (adapter-node).

One command to run everything: docker compose up.

A GitHub Actions pipeline that runs lint, tests, and image build.

Optional: deploy to a cheap VPS or Fly.io.

What makes it interview-worthy
Your README matters as much as the code. Include:

An architecture diagram, and a short “why Postgres as queue, and where it stops scaling” section.

A load test (k6 or vegeta) with real numbers, such as events/sec and p99 attempt latency.

A two-minute “incident story”: turn on the chaos receiver, watch queue lag climb, find the slow endpoint in a trace, then explain the fix, such as a per-endpoint timeout or concurrency cap. Interviewers remember a story like this.

A short list of known limitations and what you’d change at 100x scale.