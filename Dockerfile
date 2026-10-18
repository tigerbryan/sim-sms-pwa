# syntax=docker/dockerfile:1

# Stage 1: Build frontend
FROM node:24-alpine AS frontend
WORKDIR /src
# We copy into /src/web since vite config outDir is ../internal/web/dist
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web/ ./web/
COPY VERSION ./VERSION
RUN cd web && npm run build

# Stage 2: Build backend
FROM golang:1.26.9-alpine AS backend
ARG VERSION=1.1.0
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
# Copy frontend build output
COPY --from=frontend /src/internal/web/dist ./internal/web/dist
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -buildvcs=false -trimpath \
  -ldflags="-s -w -X qmi-web/internal/version.Version=${VERSION} -X qmi-web/internal/version.Commit=${COMMIT} -X qmi-web/internal/version.BuildTime=${BUILD_TIME}" \
  -o /out/qmi-web ./cmd/server

# Stage 3: Minimal runtime
FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=1.1.0
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.title="QMI Web" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.licenses="MIT"
WORKDIR /
COPY --from=backend /out/qmi-web /qmi-web
USER nonroot:nonroot
EXPOSE 7580
ENTRYPOINT ["/qmi-web"]
