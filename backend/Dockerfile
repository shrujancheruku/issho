# One image for the API, the worker and the tool (migrations, seeding):
# Kamal runs the API by default and the worker with `cmd: worker` (see
# config/deploy.yml). Build from the repository root, since the tool's seed
# reads data/:
#   docker build -f backend/Dockerfile --build-arg VERSION=$(git rev-parse --short HEAD) .

# The build stage runs on the builder's own platform and cross-compiles, so
# an amd64 image builds quickly on an arm64 Mac (no emulation).
# Alpine: a much smaller pull than the Debian image, and with cgo off the
# libc doesn't matter.
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build
WORKDIR /src/backend
ARG VERSION=dev
ARG TARGETOS TARGETARCH
# No `go mod download` layer: go.mod's `tool` block (buf, sqlc, …) makes it
# fetch ~370 modules when the binaries need ~60, which was most of a cold
# build. `go build` fetches only what it imports. The module and build
# caches are BuildKit cache mounts, so they outlive the layer: a code change
# downloads nothing and recompiles only the packages that changed. The
# source is bind-mounted rather than copied, since only /out is kept. CI
# fills the same cache mounts (the image job in .github/workflows/ci.yml);
# keep the paths in step.
# Static binaries (no cgo), so the runtime image needs no libc.
RUN --mount=type=bind,source=backend,target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ ./cmd/api ./cmd/worker ./cmd/tool

# distroless: CA certificates and tzdata (time zones for city packs), no
# shell, runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
# The tool's seed defaults are ../data/…, relative to /app.
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY data/ /data/
ENV API_ADDR=:8080
EXPOSE 8080
CMD ["api"]
