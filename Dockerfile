# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /goflow-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /goflow-worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /goflow-scheduler ./cmd/scheduler

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binaries
COPY --from=builder /goflow-server /app/goflow-server
COPY --from=builder /goflow-worker /app/goflow-worker
COPY --from=builder /goflow-scheduler /app/goflow-scheduler

# Copy config template
COPY config.example.yaml /app/config.example.yaml

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -q --spider http://localhost:8080/health || exit 1

# Default: run server (override with command in docker-compose/stack)
ENTRYPOINT ["/app/goflow-server"]
CMD ["-port", "8080"]
