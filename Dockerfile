# syntax=docker/dockerfile:1
# One image for every Go mode; compose picks the mode with `command`.

# --- build: compile a fully static binary ---
FROM golang:1.27 AS build
WORKDIR /src

# Download modules in their own layer so code edits don't refetch them.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd ./cmd
COPY internal ./internal

# CGO_ENABLED=0: pure-Go DNS and user lookup, so the binary links no libc and
# runs on distroless/static. -trimpath drops local paths; -s -w strips symbols.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hookline ./cmd/hookline

# --- runtime: no shell, no libc, non-root (uid 65532) ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/hookline /hookline
USER nonroot:nonroot
ENTRYPOINT ["/hookline"]
CMD ["api"]
