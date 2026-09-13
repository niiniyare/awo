> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Upgrade Guide"
id: ops-005
status: accepted
category: GUIDE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: informative
related:
  - "[Migrations](migrations.md)"
  - "[Deployment](deployment.md)"
  - "[Versioning Policy](../00-documentation/versioning-policy.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Upgrade Guide

**OPS-005 | Status: Accepted | Stability: Stable**

Procedures for upgrading Awo to a new version in production.

---

## 1. Before Upgrading

1. **Read the changelog** for the target version — check for breaking changes, migration requirements, and deprecation notices.
2. **Test in staging first** — apply migrations and run the new version in a staging environment with production-like data volume.
3. **Back up the database** — take a PostgreSQL base backup before any production upgrade.
4. **Check amis SDK compatibility** — if the new version bumps the pinned amis SDK, review all custom `PageBuilderSet` outputs for compatibility.

---

## 2. Upgrade Procedure (Zero-Downtime)

### Step 1: Apply Migrations

Migrations run as a Kubernetes Job before the server rollout:

```bash
kubectl apply -f deploy/migrate-job.yaml
kubectl wait --for=condition=complete job/awo-migrate --timeout=300s
```

Verify no migration errors:

```bash
kubectl logs job/awo-migrate
```

If the migration job fails:
- Check `schema_migrations` for `dirty = true`
- Do not proceed with server rollout until migrations are clean
- See [Migrations §6](migrations.md#6-dirty-state-recovery) for recovery

### Step 2: Roll Out New Server Version

```bash
kubectl set image deployment/awo-server awo-server=awo:v1.2.3
kubectl rollout status deployment/awo-server
```

Kubernetes rolls out pods one by one (`maxUnavailable: 0`). Traffic only routes to pods that pass the readiness probe.

### Step 3: Verify

```bash
# Check all pods running new version
kubectl get pods -l app=awo-server -o jsonpath='{.items[*].spec.containers[0].image}'

# Check readiness
kubectl get endpoints awo-server

# Check error rate (first 5 minutes after rollout)
# Use Prometheus or Grafana
```

### Step 4: Roll Back if Needed

```bash
kubectl rollout undo deployment/awo-server
```

**Note**: Rolling back the application does not roll back database migrations. If the new version's migrations are incompatible with the old application version, the old version may fail.

Migration-safe rollback requires the migration to be backward-compatible with the previous application version. This is why Awo uses additive-only migrations (no column renames, no column drops in the same migration as the code change).

---

## 3. Migration Compatibility Matrix

| Migration type | Old app compatible? | Notes |
|---|---|---|
| Add column (nullable) | Yes | Old app ignores new column |
| Add index | Yes | No schema change visible to app |
| Add table | Yes | Old app doesn't know about new table |
| Add enum value to CHECK constraint | No — if old app never sends new value | Extend CHECK before deploying app that uses new value |
| Drop column | No | Drop only after old app version no longer references the column |
| Rename column | No | Multi-step: add new, dual-write, migrate reads, drop old |
| Change column type | No | Always additive — add new column, migrate, drop old |

---

## 4. Temporal Worker Upgrade

The Temporal worker and HTTP server are deployed together in the same process. Worker upgrade follows the same rollout procedure.

**Workflow versioning**: if a new release changes workflow logic for an in-flight workflow type, use Temporal's `workflow.GetVersion` API to maintain backward compatibility:

```go
func InvoiceApprovalWorkflow(ctx workflow.Context, input ApprovalInput) error {
    version := workflow.GetVersion(ctx, "notify-before-approval", workflow.DefaultVersion, 1)

    if version == workflow.DefaultVersion {
        // old path: approve first, notify after
    } else {
        // new path: notify first, then wait for approval
        _ = workflow.ExecuteActivity(ctx, activities.NotifyApproverActivity, input).Get(ctx, nil)
    }
    // ...
}
```

Without versioning, replaying old workflow histories with new code causes non-determinism errors.

---

## 5. amis SDK Upgrade

The amis SDK is pinned in `web/sdk/`. Upgrading requires:

1. Download new SDK files to `web/sdk/`
2. Update version reference in `web/pages/index.html`
3. Run the full page builder compatibility test suite
4. Visually inspect all auto-generated pages and custom `PageBuilderSet` pages
5. Fix any rendering regressions
6. Deploy

**Never auto-update the amis SDK** — minor amis versions have broken component APIs. This step requires a full manual review. See ADR-003.

---

## 6. Go Module Updates

```bash
# View available updates
go list -u -m all

# Update specific module
go get awo.so/dependency@v1.2.3

# Update all to latest patch versions (low risk)
go get -u=patch ./...

# Audit for known vulnerabilities
govulncheck ./...
```

Always review the diff after any dependency update. Run the full test suite before deploying.

---

## 7. Version Pinning Reference

| Component | Pinned? | Location | Notes |
|---|---|---|---|
| amis SDK | Yes | `web/sdk/` | Never auto-update |
| Go modules | Yes | `go.sum` | Update deliberately |
| PostgreSQL | Minor only | Terraform/Helm | Major upgrades need testing |
| Redis | Minor only | Terraform/Helm | |
| Temporal | Yes | Go module | API changes between versions |
| PgBouncer | Yes | Docker image | |

---

## Related Documents

- [Migrations](migrations.md) — migration process and dirty state recovery
- [Deployment](deployment.md) — Kubernetes deployment manifest
- [Versioning Policy](../00-documentation/versioning-policy.md) — SemVer, breaking change definition
- [ADR-003](../17-adr/adr-003-amis-sdui.md) — rationale for amis SDK pinning
