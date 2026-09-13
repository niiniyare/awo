# Kubernetes Deployment

**Classification:** Guide — Tier 2
**Owner:** `20-devops/KUBERNETES_DEPLOYMENT.md`
**Status:** Living document

---

## Architecture

```
Internet
    │
    ▼
Load Balancer (HTTPS)
    │
    ▼
Ingress Controller (nginx/Traefik)
    │
    ▼
awo-erp-server Deployment (3+ replicas)
    │         │           │
    ▼         ▼           ▼
PostgreSQL  Redis     Temporal
(RDS/Cloud) (ElastiCache) (Temporal Cloud / self-hosted)
```

---

## Deployment Spec

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: awo-erp-server
  namespace: production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: awo-erp-server
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0   # zero-downtime rolling deploy
  template:
    metadata:
      labels:
        app: awo-erp-server
    spec:
      containers:
      - name: awo-erp-server
        image: awo-erp:${VERSION}
        ports:
        - containerPort: 8080  # API
        - containerPort: 9090  # Metrics
        envFrom:
        - secretRef:
            name: awo-erp-secrets
        - configMapRef:
            name: awo-erp-config
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
          failureThreshold: 2
```

---

## Migration Job

Migrations run as a Kubernetes Job before the rolling deploy:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: awo-erp-migrate-${VERSION}
  namespace: production
spec:
  backoffLimit: 2
  template:
    spec:
      restartPolicy: OnFailure
      containers:
      - name: migrate
        image: awo-erp:${VERSION}
        command: ["/app/awoerp", "migrate", "up"]
        envFrom:
        - secretRef:
            name: awo-erp-secrets
```

Deploy order: `apply migration job` → wait for completion → `apply deployment rollout`.

---

## PgBouncer (Required)

PgBouncer MUST be deployed in transaction mode between the application and PostgreSQL. Session mode breaks RLS.

```yaml
# PgBouncer ConfigMap
data:
  pgbouncer.ini: |
    [databases]
    awo = host=postgres-primary port=5432 dbname=awo

    [pgbouncer]
    pool_mode = transaction     # REQUIRED — session mode breaks RLS
    max_client_conn = 200
    default_pool_size = 25
    server_idle_timeout = 300
```

---

## Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: awo-erp-server
  namespace: production
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: awo-erp-server
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: awo-erp
  namespace: production
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "30"
spec:
  tls:
  - hosts:
    - api.example.com
    secretName: awo-erp-tls
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: awo-erp-server
            port:
              number: 8080
```

Internal endpoints (`/metrics`, `/health/*`) are NOT exposed via ingress — accessed only within the cluster.

---

## References

- [`20-devops/ENVIRONMENT_VARIABLES.md`](ENVIRONMENT_VARIABLES.md) — Environment configuration
- [`20-devops/CICD_PIPELINE.md`](CICD_PIPELINE.md) — CI/CD pipeline
- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — PgBouncer transaction mode requirement
