# CI/CD Pipeline

**Classification:** Guide — Tier 2
**Owner:** `20-devops/CICD_PIPELINE.md`
**Status:** Living document

---

## Pipeline Overview

```
Push to branch
    │
    ▼
[CI] Test + Lint + Build
    │
    ▼
[CI] Integration Tests (real PostgreSQL)
    │
    ▼
Merge to main
    │
    ▼
[CD] Build Docker image + push to registry
    │
    ▼
[CD] Run migration job (staging)
    │
    ▼
[CD] Rolling deploy to staging
    │
    ▼
Manual approval
    │
    ▼
[CD] Run migration job (production)
    │
    ▼
[CD] Rolling deploy to production
```

---

## CI Checks (on every PR)

```yaml
# .github/workflows/ci.yml
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_USER: awo
          POSTGRES_PASSWORD: awo_ci
          POSTGRES_DB: awo_ci
        options: >-
          --health-cmd pg_isready
          --health-interval 5s
          --health-retries 10
      redis:
        image: redis:7-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 5s
          --health-retries 10
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Unit tests
        run: go test ./...
      - name: Integration tests
        run: go test -tags=integration ./...
        env:
          DATABASE_URL: postgres://awo:awo_ci@localhost:5432/awo_ci?sslmode=disable
          REDIS_URL: redis://localhost:6379/0
      - name: Vet
        run: go vet ./...
      - name: Migration lint
        run: |
          # Verify all .up.sql have corresponding .down.sql
          for f in db/migration/*.up.sql; do
            base="${f%.up.sql}"
            if [ ! -f "${base}.down.sql" ]; then
              echo "MISSING DOWN: ${base}.down.sql"
              exit 1
            fi
          done
```

---

## CD: Docker Build

```yaml
# .github/workflows/deploy.yml (on push to main)
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build Docker image
        run: |
          docker build -t awo-erp:${GITHUB_SHA::8} .
          docker tag awo-erp:${GITHUB_SHA::8} registry.example.com/awo-erp:${GITHUB_SHA::8}
          docker push registry.example.com/awo-erp:${GITHUB_SHA::8}
```

---

## Dockerfile

```dockerfile
FROM golang:1.22-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o awoerp ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/awoerp .
COPY web/ ./web/
EXPOSE 8080 9090
ENTRYPOINT ["./awoerp"]
```

---

## CD: Deploy Sequence

```bash
# 1. Run migration job and wait
kubectl apply -f k8s/jobs/migrate.yaml
kubectl wait --for=condition=complete job/awo-erp-migrate --timeout=300s

# 2. Rolling deploy
kubectl set image deployment/awo-erp-server \
  awo-erp-server=registry.example.com/awo-erp:${VERSION}

# 3. Wait for rollout
kubectl rollout status deployment/awo-erp-server --timeout=300s

# 4. Smoke test
curl -sf https://api.example.com/health/ready
```

If `kubectl rollout status` fails → automatic rollback:

```bash
kubectl rollout undo deployment/awo-erp-server
```

---

## Branch Strategy

| Branch | Triggers | Environment |
|--------|----------|-------------|
| `feature/*` | CI only | — |
| `main` | CI + CD → staging | Staging |
| `main` (manual approval) | CD → production | Production |

---

## References

- [`20-devops/KUBERNETES_DEPLOYMENT.md`](KUBERNETES_DEPLOYMENT.md) — Kubernetes spec
- [`20-devops/MONITORING.md`](MONITORING.md) — Post-deploy monitoring
