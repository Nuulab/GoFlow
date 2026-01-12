---
title: Build all binaries
---

title: Deployment
---


Deploy GoFlow to production.

## Single Binary

Build and run:

```bash
# Build all binaries
go build -o goflow-server ./cmd/server
go build -o goflow-worker ./cmd/worker
go build -o goflow-scheduler ./cmd/scheduler

# Run
./goflow-server -port 8080
./goflow-worker -concurrency 10
./goflow-scheduler
```

## Docker

```bash
# Build image
docker build -t goflow .

# Run with environment
docker run -p 8080:8080 \
  -e GOFLOW_REDIS=redis:6379 \
  -e OPENAI_API_KEY=sk-... \
  goflow
```

## Docker Compose

```yaml
version: "3.8"

services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      - GOFLOW_REDIS=dragonfly:6379
    depends_on:
      - dragonfly

  worker:
    build: .
    command: ["/app/goflow-worker", "-concurrency", "10"]
    environment:
      - GOFLOW_REDIS=dragonfly:6379
    deploy:
      replicas: 3

  dragonfly:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    ports:
      - "6379:6379"
    volumes:
      - dragonfly_data:/data

volumes:
  dragonfly_data:
```

## Docker Swarm

Production deployment with high availability:

```bash
# Initialize swarm
docker swarm init

# Deploy
docker stack deploy -c docker-stack.yml goflow

# Scale
docker service scale goflow_worker=10
docker service scale goflow_api=3
```

## Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: goflow-api
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: api
          image: goflow:latest
          ports:
            - containerPort: 8080
          env:
            - name: GOFLOW_REDIS
              value: redis:6379
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: goflow-worker
spec:
  replicas: 10
  template:
    spec:
      containers:
        - name: worker
          image: goflow:latest
          command: ["/app/goflow-worker"]
          args: ["-concurrency", "5"]
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GOFLOW_PORT` | API server port | 8080 |
| `GOFLOW_REDIS` | Redis/DragonflyDB address | localhost:6379 |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `ANTHROPIC_API_KEY` | Anthropic API key | - |
| `GOFLOW_WORKER_CONCURRENCY` | Workers per instance | 5 |

## Health Checks

```bash
# API health
curl http://localhost:8080/health

# Ready check (includes Redis)
curl http://localhost:8080/ready
```

## Reverse Proxy (Nginx)

```nginx
upstream goflow {
    server goflow-api-1:8080;
    server goflow-api-2:8080;
    server goflow-api-3:8080;
}

server {
    listen 80;
    
    location / {
        proxy_pass http://goflow;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## TLS

```bash
./goflow-server \
  -port 443 \
  -tls-cert /path/to/cert.pem \
  -tls-key /path/to/key.pem
```

## Logging

```bash
# JSON logs for production
./goflow-server -log-format json

# Debug logging
./goflow-server -verbose
```

## Backup & Recovery

```bash
# Backup DragonflyDB
docker exec dragonfly redis-cli BGSAVE

# Restore
docker cp backup.rdb dragonfly:/data/dump.rdb
docker restart dragonfly
```
