# Local values come from .env (copy .env.example). Variables are exported to
# recipes only, never to your interactive shell.
ifneq (,$(wildcard .env))
include .env
export
endif

GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
PG_CONTAINER  ?= hookline-pg
PG_IMAGE      ?= postgres:17

.PHONY: help fmt fmt-check vet lint test build run-api run-worker run-chaos db-up db-down db-psql \
	web-install web-dev web-check web-lint web-test web-build web-run

help:
	@grep -E '^[a-z-]+:' Makefile | cut -d: -f1

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

lint:
	$(GOLANGCI_LINT) run

test:
	go test -race -count=1 ./...

build:
	go build -o bin/hookline ./cmd/hookline

run-api: build
	./bin/hookline api

run-worker: build
	./bin/hookline worker

run-chaos: build
	./bin/hookline chaos

db-up:
	@docker start $(PG_CONTAINER) 2>/dev/null || docker run -d --name $(PG_CONTAINER) \
		-e POSTGRES_USER=hookline -e POSTGRES_PASSWORD=hookline -e POSTGRES_DB=hookline \
		-p 5432:5432 $(PG_IMAGE)
	@until docker exec $(PG_CONTAINER) pg_isready -U hookline -q; do sleep 0.5; done
	@echo "postgres ready"

db-down:
	docker rm -f $(PG_CONTAINER)

# Usage: make db-psql Q='select 1'
db-psql:
	docker exec $(PG_CONTAINER) psql -U hookline -d hookline -c "$(Q)"

# --- web (SvelteKit, in web/) ---
# web-dev reads web/.env (copy web/.env.example). web-run needs API_URL in the
# environment, e.g. make web-run API_URL=http://localhost:8080
WEB_NPM := npm --prefix web

web-install:
	$(WEB_NPM) ci

web-dev:
	$(WEB_NPM) run dev

web-check:
	$(WEB_NPM) run check

web-lint:
	$(WEB_NPM) run lint

web-test:
	$(WEB_NPM) test

web-build:
	$(WEB_NPM) run build

web-run: web-build
	node web/build
