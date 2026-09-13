> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Backup and Restore"
id: ops-006
status: accepted
category: GUIDE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: normative
related:
  - "[Deployment](deployment.md)"
  - "[Migrations](migrations.md)"
  - "[Security Model](../15-security/security-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Backup and Restore

**OPS-006 | Status: Accepted | Stability: Stable**

Procedures for PostgreSQL backup, Redis backup, and full system restore.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. PostgreSQL Backup

### Continuous WAL Archiving (Production)

Production deployments MUST use WAL archiving for point-in-time recovery:

```bash
# postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'aws s3 cp %p s3://awo-backups/wal/%f'
```

Combined with daily base backups:

```bash
# Daily base backup via pg_basebackup
pg_basebackup \
  --host=$PGHOST \
  --username=$PGUSER \
  --pgdata=/tmp/pg_backup_$(date +%Y%m%d) \
  --format=tar \
  --gzip \
  --checkpoint=fast

# Upload to S3
aws s3 cp /tmp/pg_backup_$(date +%Y%m%d).tar.gz s3://awo-backups/base/
```

This enables recovery to any point in time within the WAL retention window.

### Logical Backup (Staging/Dev)

For staging environments or per-database snapshots:

```bash
pg_dump \
  --host=$PGHOST \
  --username=$PGUSER \
  --dbname=awo \
  --format=custom \
  --file=awo_$(date +%Y%m%d_%H%M%S).dump

# Restore
pg_restore \
  --host=$PGHOST \
  --username=$PGUSER \
  --dbname=awo_restored \
  --format=custom \
  awo_20240315_090000.dump
```

### Backup Retention

| Backup type | Retention |
|---|---|
| WAL archives | 7 days |
| Daily base backups | 30 days |
| Monthly snapshots | 12 months |
| Pre-upgrade snapshots | 90 days |

Retention requirements may be extended by tenant contracts or Kenya DPA 2019 compliance requirements.

---

## 2. Redis Backup

Redis in Awo stores session tokens, feature flag evaluations, and page schema caches. All data is ephemeral:

- Session tokens: re-created on login (users re-authenticate after Redis restore)
- Feature flags: re-evaluated from PostgreSQL on first miss after restore
- Page schemas: re-generated on first request after restore

Redis backup is RECOMMENDED for session continuity but NOT REQUIRED for data integrity. All authoritative data is in PostgreSQL.

```bash
# Enable Redis RDB persistence
redis-cli CONFIG SET save "3600 1 300 100 60 10000"

# Manual snapshot
redis-cli BGSAVE
```

Redis AOF (Append-Only File) provides stronger durability but higher I/O. Enable for environments where session loss is unacceptable:

```
appendonly yes
appendfsync everysec
```

---

## 3. Point-in-Time Restore

To restore PostgreSQL to a specific point in time (e.g., before an accidental bulk delete):

```bash
# 1. Stop the application
kubectl scale deployment awo-server --replicas=0

# 2. Restore base backup
aws s3 cp s3://awo-backups/base/pg_backup_20240315.tar.gz /tmp/
tar xzf /tmp/pg_backup_20240315.tar.gz -C /var/lib/postgresql/data

# 3. Create recovery.conf (PostgreSQL < 12) or postgresql.auto.conf
cat >> /var/lib/postgresql/data/postgresql.auto.conf <<EOF
recovery_target_time = '2024-03-15 09:30:00'
recovery_target_action = promote
restore_command = 'aws s3 cp s3://awo-backups/wal/%f %p'
EOF

# 4. Start PostgreSQL — it will apply WAL up to the target time
pg_ctl start -D /var/lib/postgresql/data

# 5. Verify data is in expected state
psql -c "SELECT count(*) FROM finance_invoice WHERE created_at < '2024-03-15 09:30:00';"

# 6. Run migrations to ensure schema is current
kubectl run --rm -it awo-migrate --image=awo:latest -- /app/migrate up

# 7. Restart application
kubectl scale deployment awo-server --replicas=3
```

All sessions are invalidated after a restore (Redis contains sessions from before the outage; PostgreSQL is restored to a point before some sessions were created — state mismatch). Users must re-authenticate.

---

## 4. Disaster Recovery RTO/RPO

| Scenario | RPO (data loss) | RTO (downtime) |
|---|---|---|
| Single pod failure | 0 | <30 seconds (Kubernetes reschedule) |
| Database failover (primary → replica) | <30 seconds (WAL lag) | 2-5 minutes |
| Full zone failure | <1 minute (WAL archiving lag) | 10-30 minutes |
| Full region failure | <1 hour (daily backup lag) | 2-4 hours |

---

## 5. Tenant Data Export

For GDPR/Kenya DPA right to data portability, individual tenant data can be exported:

```
POST /api/v1/admin/tenants/{tenant_id}/export
Authorization: Bearer {platform_admin_token}
```

This triggers a Temporal workflow that:
1. Dumps all tenant-scoped tables to JSON
2. Packages into a zip archive
3. Stores in S3 with a pre-signed URL (valid 24 hours)
4. Notifies the platform admin via email when ready

The export includes all entity data but excludes: `password_hash`, `session_token`, and other fields marked `Sensitive: true`.

---

## 6. Pre-Upgrade Backup

Before any production upgrade:

```bash
# Snapshot before upgrade
pg_dump --format=custom --file=pre_upgrade_$(git describe --tags).dump awo

# Tag the backup with the version being upgraded FROM
aws s3 cp pre_upgrade_v1.2.3.dump \
  s3://awo-backups/upgrades/pre_v1.2.3_$(date +%Y%m%d).dump
```

Retain pre-upgrade snapshots for 90 days minimum — sufficient to restore if a silent data corruption is discovered post-upgrade.

---

## Related Documents

- [Deployment](deployment.md) — Kubernetes deployment and health probes
- [Upgrade Guide](upgrade-guide.md) — pre-upgrade backup procedure
- [Troubleshooting](troubleshooting.md) — recovery from corruption or migration failures
- [Security Model](../15-security/security-model.md) — data residency and compliance notes
