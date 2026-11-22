# One image for both backend processes, `api` and `worker`: they share the
# module, the migrations and the release, so they ship and roll back together.
# The deploy picks the process by its command (`/usr/local/bin/api`, the
# default, or `/usr/local/bin/worker`). Build from the backend/ directory:
#
#   docker build --file backend/Dockerfile --build-arg VERSION=1.2.3 backend
#
# Both base images are pinned by digest; Renovate keeps them current.

# The Go toolchain matches the go directive in go.mod.
FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
WORKDIR /src

# The module cache is its own layer, so a source change doesn't download again.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# VERSION is what the API's health check reports (`main.version`); the worker
# has no such variable and ignores the flag.
ARG VERSION=dev
# A static binary (no cgo) runs on the runtime image, which has no libc.
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ ./cmd/api ./cmd/worker

# distroless/static has no shell or package manager and runs as a non-root user.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931
# Links the package to this repository, so the deploy workflow's GITHUB_TOKEN may push it.
LABEL org.opencontainers.image.source="https://github.com/shrujancheruku/issho"
COPY --from=build /out/api /out/worker /usr/local/bin/
CMD ["/usr/local/bin/api"]
