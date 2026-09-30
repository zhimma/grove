# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.27.1
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .

# Keep service selection after dependencies so all three binaries share them.
ARG SERVICE=api
RUN case "${SERVICE}" in \
      api|console|worker) ;; \
      *) echo "unsupported SERVICE: ${SERVICE}" >&2; exit 1 ;; \
    esac
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "/out/grove-${SERVICE}" "./app/${SERVICE}/cmd"

FROM debian:12.9-slim

ARG SERVICE=api
ENV TZ=UTC

RUN apt-get update \
    && apt-get install --no-install-recommends --yes ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 grove \
    && useradd --system --uid 10001 --gid 10001 --home-dir /nonexistent --shell /usr/sbin/nologin grove \
    && install -d -o grove -g grove -m 0750 /app /app/logs /app/storage

COPY --from=builder --chown=grove:grove "/out/grove-${SERVICE}" /app/grove

WORKDIR /app
USER grove:grove
EXPOSE 8080 8081 8082

# Mount a protected config.yaml at /app/config.yaml in the deployment
# environment. The image intentionally contains no usable credentials.
ENTRYPOINT ["/app/grove"]
