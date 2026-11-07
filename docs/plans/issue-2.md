# Plan — Issue #2: SvelteKit skeleton with "API: ok" health tile

Issue: https://github.com/anrisys/hookline/issues/2 · Milestone: S0 · Branch: `issue-2-sveltekit-skeleton`

Sources: `requirements.md` (Scope, R18, R21, R22, N5), `architecture.md` (Components, Code layout, Configuration), `backlog.md` (S0), `plans/issue-1.md` (format, `/healthz` contract).

## A. Acceptance criteria
- **AC1** `web/` is a SvelteKit 3 app (Svelte 5, TypeScript, adapter-node) with eslint and svelte-check passing.
- **AC2** A server-only API client reads `API_URL` / `API_KEY` from env; the key never reaches the browser (R18).
- **AC3** The home page `load` fetches `/healthz` server-side and shows "API: ok" with the API up and "API: down" with it stopped.

One session. Docker/compose is #3, CI is #4.

## B. Plan
| Area | Change |
|---|---|
| Backend | None. `/healthz` exists (`internal/httpx/httpx.go`): 200 `{"status":"ok"}` / 503 `{"status":"unavailable"}`, no auth. Server-to-server call, so no CORS. |
| Database | None |
| Frontend | `web/` from `npx sv create` (minimal, TS). `src/env.ts` (`defineEnvVars`: `API_URL` required URL, `API_KEY` optional, both private and runtime), `src/lib/server/api.ts` (`apiFetch`), `src/lib/server/health.ts` (`getHealth`), `src/routes/+page.server.ts`, `src/routes/+page.svelte`, `web/.env.example`, tests beside sources. |
| Infra | `Makefile`: `web-install`, `web-dev`, `web-check`, `web-lint`, `web-test`, `web-build`. `.gitignore` covers `web/node_modules`, `web/build`, `web/.svelte-kit`. |

Design:
- **Runtime env, not build-time.** Variables declared with `static: false`, so one image (#3) works with any `API_URL`.
- **Server-only boundary.** Client and health code live under a `server/` directory, which Kit 3 refuses to bundle into browser code.
- **`apiFetch(fetch, path, init)`** prefixes `API_URL`, adds `Authorization: Bearer` when `API_KEY` is set, and aborts after 2s (longer than the API's 1s ping timeout, so a 503 is seen as a 503).
- **Never prerendered;** the load runs on every request, so a reload shows the current state.

## C. Ownership
All **AI scaffolds** (config, wiring, a trivial status mapping). Nothing here is schema, claim, retry, signing, idempotency or tracing (traceparent forwarding is #20).

## D. Tests (Vitest)
**`getHealth` (fake fetch):** 200 + `{"status":"ok"}` → ok · 503 → down · fetch rejects → down, no throw · hangs past timeout → down · 200 with non-JSON or other status → down · 500/404 → down · URL is `${API_URL}/healthz` even with a trailing slash · ok → fail → ok sequence (no caching).

**`apiFetch`:** Bearer header sent when `API_KEY` set; no header when empty.

**Env validator:** `API_URL` missing / empty / not a URL → error; valid http(s) URL → ok.

**Page:** server-render `+page.svelte` with `svelte/server` → contains "API: ok" / "API: down". Load returns only `{ api }`; the key never appears in it.

| AC | Check |
|---|---|
| AC1 | `make web-check web-lint web-build` |
| AC2 | apiFetch tests, server-only build guard, grep of `build/client` for the key |
| AC3 | getHealth + render tests, manual run with API up then down |

## E. Commits
1. `chore(web): scaffold SvelteKit 3 with TS, adapter-node, eslint, prettier, vitest`
2. `feat(web): server-only API client reading API_URL/API_KEY`
3. `feat(web): home page health tile from /healthz`
4. `chore: Makefile web targets and gitignore`

PR body includes `Closes #2`.

## F. Verify
```bash
make web-install
make web-check web-lint web-test web-build
make db-up && make run-api &                                         # API on :8080
(cd web && API_URL=http://localhost:8080 API_KEY=devkey node build &) # :3000
curl -s localhost:3000 | grep -o 'API: [a-z]*'      # API: ok
kill %1                                             # stop the API
curl -s localhost:3000 | grep -o 'API: [a-z]*'      # API: down
grep -r devkey web/build/client || echo "key not in client bundle"
(cd web && API_URL=http://localhost:9999 PORT=3001 node build &)     # same build, other URL → down
(cd web && env -u API_URL node build); echo "exit=$?"                # non-zero
```
No `docker compose` check: the stack isn't touched (#3).

## G. Decisions (resolved)
1. **Framework version: SvelteKit 3** (see below).
2. npm (pnpm not installed). Node 24, `engines.node >= 22.17`.
3. Prettier included; eslint uses the prettier config.
4. Plain scoped CSS, no Tailwind.
5. AI writes everything in this issue.
6. Tile is binary: 200 + `{"status":"ok"}` → ok, anything else → down. A DB outage (503) shows "API: down".
7. Missing/invalid `API_URL` fails at startup via the `src/env.ts` schema.
8. Refresh on reload only; live updates arrive with SSE in #16.
9. Vitest with server-side rendering; no Playwright yet.

Assumptions: adapter-node default port 3000; `API_KEY` sent already though `/healthz` ignores it, so #5 needs no wiring change.

### SvelteKit 3 vs SvelteKit 2
Checked against the official changelog (3.0.0–3.1.1) and the 3.1.1 package on 2026-11-07. 3.0.0 shipped 2026-10-01; 2.x now gets maintenance releases (2.70.x).

**SvelteKit 3 — pros**
- Active line; starting on 2 means migrating mid-project.
- Server tracing is stable and built in (moved out of `experimental`; 3.1 adds `sveltekit.endpoint` spans). Directly serves R22, N1 and #20.
- Typed, validated env vars (`defineEnvVars` in `src/env.ts`, imported from `$app/env/private`): the app refuses to start on a bad config, matching the Go side's fail-fast config.
- Stricter server-only boundary: any `/server/` directory is server-only, which protects `API_KEY` (R18).
- Vite 8 with Rolldown (faster builds); Node ≥ 22, which we have.

**SvelteKit 3 — cons**
- About five weeks old; patches still land (3.1.1 fixed a server-only import leak via `src/params.js` and a `//` route-resolution issue).
- Most tutorials and AI training data describe Kit 2. Renames: `$lib` → `#lib`, `$env/*` → `$app/env/*` (old names are deprecated aliases), `$app/stores` removed, `invalidateAll` → `refreshAll`, `goto` options changed.
- Ecosystem may lag (eslint-plugin-svelte, svelte-check, test tooling on Vite 8 / TS 6).
- More breaking changes affect later issues, e.g. adapter-node's `ORIGIN` env var is replaced by `kit.paths.origin` (form-action CSRF behind compose in #3/#6).

**SvelteKit 2 — pros:** mature, well documented, universal add-on support, examples work as written.
**SvelteKit 2 — cons:** maintenance-only, tracing still experimental, weaker env handling, guaranteed migration later.

**Why Kit 3:** `web/` is empty, so there's no migration cost; the app is small (SSR pages, form actions, one SSE proxy) and unlikely to hit rough edges; tracing is a headline feature (S7) and Kit 3 supports it natively; learning the current API costs the same as learning the old one.

**Risk control:** pin exact versions and commit `package-lock.json`; commit 1 proves the toolchain (scaffold, eslint, svelte-check, vitest, build) and we fall back to 2.70 if it can't pass cleanly; verify Kit 3 APIs against the installed package, not memory.

## H. How tests could pass wrongly
- Fake fetch everywhere → no real network, URL join or env loading; covered by the manual up/down curl.
- `static: true` env → URL baked in at build; caught by running the same build with another `API_URL`.
- Tile fetched in the browser → render test feeds `data` directly; caught by curling raw HTML (no JS) and by using `+page.server.ts`.
- Key in client bundle → caught by the server-only guard and grep of `build/client`.
- Status-code-only check → fails the 200 + wrong-body test.
- Prerendered or cached page → caught by the manual stop-the-API reload.
- Timeout never fires → the hang test must wait on real/short timers; the 200 test guards against "always down".
