> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Deployment"
id: ops-002
status: accepted
category: SPEC
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: normative
related:
  - "[Migrations](migrations.md)"
  - "[Configuration](../12-configuration/configuration.md)"
  - "[Observability](../13-observability/observability.md)"
  - "[Temporal Integration](../09-workflow/temporal-integration.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Deployment

**OPS-002 | Status: Accepted | Stability: Stable**

This document specifies container build, Kubernetes manifests, rolling deploy procedures, Temporal worker lifecycle, and process shutdown behavior.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Process Architecture

Awo runs as two processes from a single container image:

| Process | Binary | Purpose |
|---|---|---|
| API Server + Temporal Worker | `cmd/server` | Serves HTTP requests and executes Temporal workflows/activities |
| Migration Runner | `cmd/migrate` | Applies database migrations; runs as a Job before server pods start |

Both binaries are built from the same image. The entrypoint is selected via the container command.

---

## 2. Container Build

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/migrate ./cmd/migrate

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/migrate /bin/migrate
COPY web/ /app/web/
# Non-root user
RUN addgroup -g 1001 awo && adduser -D -u 1001 -G awo awo
USER awo
EXPOSE 8080
CMD ["/bin/server"]
```

The image MUST:
- Use a minimal base image (`alpine` or `distroless`)
- Run as a non-root user
- Include `ca-certificates` for TLS to external services (Temporal, SMTP)
- Include `tzdata` for timezone handling

---

## 3. Kubernetes Deployment

### Server Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: awo-server
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0  # Zero downtime: never reduce capacity during rollout
  template:
    spec:
      containers:
        - name: server
          image: awo-server:${VERSION}
          ports:
            - containerPort: 8080
          env:
            - name: PORT
              value: "8080"
            - name: ENVIRONMENT
              value: "production"
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: awo-secrets
                  key: database-url
            # ... other env vars from secrets and configmaps
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 5
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "1000m"
          terminationGracePeriodSeconds: 30
```

### Migration Job

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: awo-migrate-${VERSION}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: awo-server:${VERSION}
          command: ["/bin/migrate", "up"]
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: awo-secrets
                  key: database-url
```

The migration Job MUST complete successfully before server pods are updated. Use a CI/CD pipeline step that gates the server rollout on Job completion.

---

## 4. Rolling Deploy Procedure

```
1. Build and push container image: awo-server:{new-version}

2. Run migration Job:
   kubectl apply -f migration-job-{new-version}.yaml
   kubectl wait --for=condition=complete job/awo-migrate-{new-version} --timeout=300s

3. Update server Deployment (triggers rolling update):
   kubectl set image deployment/awo-server server=awo-server:{new-version}

4. Monitor rollout:
   kubectl rollout status deployment/awo-server

5. Verify health:
   kubectl get pods -l app=awo-server
   # All pods should show READY 1/1

6. Verify readiness endpoint returns new schema_hash:
   curl https://api.example.com/health/ready | jq .schema_hash
```

### Zero-Downtime Guarantee

The `maxUnavailable: 0` setting ensures no capacity reduction during rollout. New pods must pass the readiness probe before old pods are terminated. Requests are never routed to pods that have not passed `/health/ready`.

During the rollout window, both old and new server versions run simultaneously. Schema changes MUST be backward-compatible with the old server version — this is the primary constraint for zero-downtime database migrations.

---

## 5. Temporal Worker Lifecycle

The Temporal worker runs within the API server process as a goroutine started after the server is ready:

```go
// cmd/server/main.go (simplified)
func main() {
    cfg := config.MustLoad()

    // ... init postgres, redis, registry.Compile(), fiber ...

    // Start HTTP server
    go fiber.Listen(fmt.Sprintf(":%d", cfg.Port))

    // Start Temporal worker (concurrent with HTTP)
    worker := temporal.NewWorker(cfg, schema)
    go worker.Run()

    // Wait for shutdown signal
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
    <-sig

    shutdown(fiber, worker)
}
```

### Worker Shutdown

On SIGTERM (Kubernetes pod termination):

```go
func shutdown(app *fiber.App, worker worker.Worker) {
    ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
    defer cancel()

    // 1. Stop accepting new HTTP requests (Fiber graceful shutdown)
    // Allow 20 seconds for in-flight HTTP requests to complete
    if err := app.ShutdownWithContext(ctx); err != nil {
        slog.Error("HTTP shutdown error", "err", err)
    }

    // 2. Stop Temporal worker (drain in-flight activities)
    // Worker.Stop() blocks until current activity executions finish
    // (up to Temporal's configured activity timeout)
    worker.Stop()

    slog.Info("shutdown complete")
}
```

`terminationGracePeriodSeconds: 30` in the Kubernetes pod spec gives 30 seconds before SIGKILL. The shutdown sequence uses 25 seconds max to leave a buffer.

### Activity Draining

In-flight Temporal activities continue executing during worker shutdown. Temporal's worker graceful shutdown:
1. Stops polling for new activities
2. Allows current activities to complete (up to their `StartToCloseTimeout`)
3. If activities exceed the grace period, they are re-scheduled on another worker instance

Ensure `terminationGracePeriodSeconds` is longer than the longest expected activity `StartToCloseTimeout`.

---

## 6. Multiple Instances (LAW-019)

All server instances MUST run the same compiled binary — the same CompiledSchema (LAW-019). This is enforced by:
- Using the same container image tag for all instances
- The readiness probe returning the `schema_hash` — monitoring can alert if hashes diverge

During a rolling deploy, the old and new schema_hash values coexist briefly (overlapping pods). This is acceptable because:
- Migrations applied before deploy are backward-compatible with old schema
- Only the UI schema (Redis-cached) differs between versions (different cache key due to version)
- No single request spans both versions

---

## 7. Infrastructure Dependencies

| Dependency | Required For | Failure Mode |
|---|---|---|
| PostgreSQL (via PgBouncer) | All data operations | Startup exits; requests 503 during operation |
| Redis | Sessions, cache | Startup exits if unreachable; all auth fails during operation |
| Temporal | Workflow dispatch | Degraded mode — CRUD works, workflows queue in outbox |

PgBouncer MUST be deployed in transaction mode. Session mode breaks RLS (the tenant context resets on COMMIT, but session mode does not destroy the connection after COMMIT).

---

## 8. Rollback

If the rolling deploy fails (pods not passing readiness probe):

```bash
# Kubernetes automatically stops the rolling update when pods fail readiness
# Manually rollback to previous version:
kubectl rollout undo deployment/awo-server

# Verify rollback complete:
kubectl rollout status deployment/awo-server
```

Database rollback (if needed — apply only the down migration for the problematic step):

```bash
./awo-migrate down 1
```

Rollback should restore the previous container image and optionally roll back the migration. The migration rollback is only safe if no new data was written in the new schema format during the failed deploy.

---

## Related Documents

- [Migrations](migrations.md) — migration job as part of deployment pipeline
- [Configuration](../12-configuration/configuration.md) — all required environment variables
- [Observability](../13-observability/observability.md) — health probes and monitoring during deploy
- [Temporal Integration](../09-workflow/temporal-integration.md) — worker lifecycle details
- [Architecture Laws](../02-architecture/laws.md) — LAW-019 (identical schema across instances)
- [Glossary](../GLOSSARY.md) — Rolling Deploy, PgBouncer, Temporal Worker
