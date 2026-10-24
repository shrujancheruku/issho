# One image for the API, the worker and the tool (migrations, seeding):
# Kamal runs the API by default and the worker with `cmd: worker` (see
# config/deploy.yml). Build from the repository root, since the tool's seed
# reads data/:
#   docker build -f backend/Dockerfile --build-arg VERSION=$(git rev-parse --short HEAD) .

# The build stage runs on the builder's own platform and cross-compiles, so
# an amd64 image builds quickly on an arm64 Mac (no emulation).
FROM --platform=$BUILDPLATFORM golang:1.27.3-bookworm AS build
WORKDIR /src/backend
# Modules first, so code changes don't re-download them.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG VERSION=dev
ARG TARGETOS TARGETARCH
# Static binaries (no cgo), so the runtime image needs no libc.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ ./cmd/api ./cmd/worker ./cmd/tool

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
