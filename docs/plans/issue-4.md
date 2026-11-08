# Plan — Issue #4: GitHub Actions CI (lint, test, image build)

Issue: https://github.com/anrisys/hookline/issues/4 · Milestone: S0 · Branch: `issue-4-ci`

Sources: `requirements.md` (Scope, N5), `backlog.md` (S0 done-when: "CI is green on a PR"), `architecture.md` (Components, Code layout), `plans/issue-1.md`–`issue-3.md`.

## A. Acceptance criteria
- **AC1** golangci-lint, plus `go test ./...` against a Postgres service container.
- **AC2** eslint (with the prettier check in `npm run lint`) and svelte-check for `web/`.
- **AC3** Both Docker images build, no push.
- **AC4** A PR shows green lint, test and build checks.
- **AC5** A deliberately broken lint fails the run.

One session.

## B. Plan
| Area | Change |
|---|---|
| Backend / DB / Frontend | None. |
| Infra | `.github/workflows/ci.yml`. Lint version cross-reference comment in `Makefile`. |

`ci.yml`, on `push` to `master` and `pull_request`; `permissions: contents: read`; concurrency per ref with cancel-in-progress; no path filters; per-job timeouts. Three parallel jobs:
- **go:** `setup-go` from `go.mod` → `make fmt-check vet` → `golangci-lint-action` pinned to the Makefile's version → `make test`. Service `postgres:17` with a `pg_isready` health check; `TEST_DATABASE_URL` points at it. The default superuser covers storetest's `CREATE DATABASE`.
- **web:** `setup-node` 24 with npm cache on `web/package-lock.json` → `make web-install web-check web-lint web-test`.
- **images:** buildx → build `.` as `hookline:ci` and `./web` as `hookline-web:ci`, `push: false`, GHA cache scoped per image.

CI reuses the Makefile targets so local and CI run the same commands.

## C. Ownership
All **AI scaffolds** (CI). The user reviews.

## D. Checks
| AC | Check |
|---|---|
| AC1 | `go` job green; log shows store/api/worker/cmd integration tests `ok`, not skipped. |
| AC1 guard | Throwaway run without `TEST_DATABASE_URL` fails via storetest's `CI` guard. |
| AC2 | `web` job green. |
| AC3 | `images` job green, both builds in the log. |
| AC4 | PR checks list shows `go`, `web`, `images` green. |
| AC5 | Throwaway commit with an unchecked error (errcheck) and an unused TS variable (eslint) turns `go` and `web` red; then dropped by force-push to the PR branch. |
| Syntax | `actionlint` clean before pushing. |

## E. Commits
1. `docs(plans): plan for issue #4 CI`
2. `ci: go job with fmt, vet, golangci-lint and tests against postgres`
3. `ci: web job with svelte-check, eslint/prettier and vitest`
4. `ci: build go and web images without push`
5. `docs(plans): issue #4 implementation notes`

## F. Verify
```bash
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
make fmt-check vet lint test && make web-check web-lint web-test
docker build -t hookline:ci . && docker build -t hookline-web:ci ./web
git push -u origin issue-4-ci && gh pr create
gh pr checks --watch
```
Compose stack untouched, so no `docker compose up` check.

## G. Decisions (all defaults accepted)
- **D1** golangci-lint via the official action pinned to `v2.14.0`; Makefile and workflow comment each other.
- **D2** Actions pinned by major tag, not SHA.
- **D3** `make smoke` in CI deferred to a follow-up.
- **D4** User enables branch protection (require `go`, `web`, `images`) after the first green run.
- **D5** actionlint run ad hoc, not a target or CI step.
- **D6** Ask before pushing, opening the PR, and the throwaway red commits.

## H. Ways the checks could pass against a wrong setup
- Integration tests silently skipped → storetest fails when `CI` is set and `TEST_DATABASE_URL` is not; guard check proves it.
- Lint version drift between Makefile and action → cross-reference comments.
- `only-new-issues` / `continue-on-error` hiding failures → not used.
- Path filters or `if:` skipping a job (skipped counts as success) → none.
- Stale GHA cache → content-keyed; Dockerfile or context change invalidates it.
- "Broken lint" that no enabled linter flags → confirm errcheck and `no-unused-vars` fail locally first.
