# Multi-stage Dockerfile for Novexa Runtime.
#
# Build:
#   docker build -t novexa:0.1.0-alpha .
#
# Run (local-first, dashboard/API on localhost inside the container):
#   docker run -p 127.0.0.1:8787:8787 -p 127.0.0.1:8788:8788 -v novexa-data:/data novexa:0.1.0-alpha
#
# The runtime stores its SQLite telemetry database under /data/.novexa/novexa.db
# because the non-root user has its home directory set to /data.

# -----------------------------------------------------------------------------
# Stage 1: build the dashboard production bundle.
# -----------------------------------------------------------------------------
FROM node:22-alpine AS dashboard-builder

WORKDIR /build/dashboard

# Install dependencies using the lockfile for reproducible builds.
COPY dashboard/package*.json ./
RUN npm ci

# Copy the dashboard source and build it.
COPY dashboard/ ./
RUN npm run build

# -----------------------------------------------------------------------------
# Stage 2: build the Go runtime binary.
# -----------------------------------------------------------------------------
FROM golang:1.25-alpine AS runtime-builder

WORKDIR /build/runtime

# Copy Go module files first to leverage the build cache.
COPY runtime/go.mod runtime/go.sum ./
RUN go mod download

# Copy the runtime source code.
COPY runtime/ ./

# Build the binary with release metadata injected via ldflags.
# modernc.org/sqlite is pure Go, so CGO can be disabled for a static binary.
ARG VERSION=0.1.0-alpha
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
      -X github.com/novexa/novexa/runtime/internal/version.Version=${VERSION} \
      -X github.com/novexa/novexa/runtime/internal/version.Commit=${COMMIT} \
      -X github.com/novexa/novexa/runtime/internal/version.BuildDate=${BUILD_DATE}" \
    -o /build/novexa ./cmd/novexa

# -----------------------------------------------------------------------------
# Stage 3: small runtime image with dashboard assets and profiles.
# -----------------------------------------------------------------------------
FROM alpine:latest

# Install the tools needed for the health check and TLS root certificates.
RUN apk add --no-cache ca-certificates wget

# Create a non-root user whose home directory is /data. This makes the default
# SQLite path (/data/.novexa/novexa.db) naturally persistent via the /data
# volume while keeping the image small and simple.
RUN adduser -D -h /data novexa

WORKDIR /novexa

# Copy the runtime binary, dashboard production bundle, and starter profiles.
COPY --from=runtime-builder /build/novexa /novexa/novexa
COPY --from=dashboard-builder /build/dashboard/dist /novexa/dashboard/dist
COPY profiles/ /novexa/profiles/
COPY README.md LICENSE CHANGELOG.md novexa.example.yaml /novexa/

# /data is where the runtime will create its local SQLite database and logs.
RUN mkdir -p /data && chown -R novexa:novexa /novexa /data
VOLUME ["/data"]

USER novexa

# Expose the API and dashboard ports. By default the runtime binds to 127.0.0.1,
# which is the container's localhost; publish the ports with `-p 127.0.0.1:...`
# when running to keep the local-first default.
EXPOSE 8787 8788

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8787/health || exit 1

ENTRYPOINT ["/novexa/novexa"]
CMD ["start", "--host", "0.0.0.0", "--dashboard-host", "0.0.0.0"]
