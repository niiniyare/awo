# Migration Architecture

## Overview

AWO uses a **framework-native, module-registered migration system** built on top of
[golang-migrate](https://github.com/golang-migrate/migrate). Each framework module
embeds its SQL files and registers them via `init()`. The engine automatically
discovers, orders, and merges all registered sources into a single migration sequence.

No manual migration numbers. No central registry file. No SQL directories to maintain.

---

## Design Goals

| Goal | How |
|------|-----|
| No manual numbering | Numbers derived from module Priority × 1000 + local step |
| Automatic discovery | `init()` registration (same pattern as `def.Register`) |
| Deterministic ordering | Topological sort by `DependsOn` + `Priority` tie-break |
| Framework extractable | All SQL embedded in Go packages via `embed.FS` |
| Backward compatible | golang-migrate tracks applied versions; stable number scheme |
| No network required | `iofs` source driver; no file I/O at runtime |

---

## Package Layout

```
awo/migration/
├── migration.go          — Source type, Register(), BuildFS(), topo sort
├── engine.go             — Runner wrapping golang-migrate
└── bootstrap/
    ├── migrations.go     — embed + Register (Priority=0)
    ├── 001_extensions.up.sql
    ├── 001_extensions.down.sql
    ├── 002_utilities.up.sql
    └── 002_utilities.down.sql

awo/platform/tenant/migrations/
├── migrations.go         — embed + Register (Priority=10)
└── 001_platform_tenant.{up,down}.sql

awo/platform/iam/migrations/
├── migrations.go         — embed + Register (Priority=20)
└── 001–007_iam_*.{up,down}.sql

modules/finance/migrations/
├── migrations.go         — embed + Register (Priority=30)
└── 001–011_finance_*.{up,down}.sql
```

---

## Module Registration

```go
// In awo/platform/iam/migrations/migrations.go:

//go:embed *.sql
var sqlFS embed.FS

func init() {
    migration.Register(migration.Source{
        Module:    "iam",
        Priority:  20,
        DependsOn: []string{"bootstrap", "tenant"},
        FS:        sqlFS,
    })
}
```

- `Module` — unique string identifier, used in `DependsOn` references.
- `Priority` — lower runs first; resolves ties when `DependsOn` doesn't specify order.
- `DependsOn` — modules that must be fully applied before this one starts.
- `FS` — embedded filesystem with `{NNN}_{description}.{up,down}.sql` files.

---

## Version Numbering

```
Global version = Priority × 1000 + LocalStep
```

| Module     | Priority | Local Step | Global Version |
|------------|----------|------------|----------------|
| bootstrap  | 0        | 001        | 1              |
| bootstrap  | 0        | 002        | 2              |
| tenant     | 10       | 001        | 10001          |
| iam        | 20       | 001        | 20001          |
| iam        | 20       | 007        | 20007          |
| finance    | 30       | 001        | 30001          |
| finance    | 30       | 011        | 30011          |

**Rules:**
- Each module has up to 999 local steps.
- Priority gaps (0, 10, 20, 30…) allow inserting future modules between existing ones.
- Once assigned, a global version never changes — versions are stable across builds.

---

## Dependency Ordering Algorithm

1. Collect all registered `Source` values.
2. Build a map of `module → Source` for O(1) lookup.
3. Validate all `DependsOn` references exist (panic on missing dep).
4. Perform depth-first topological sort:
   - Sorted iteration order: Priority ASC, Module name ASC (deterministic).
   - Visit each dep recursively before the depending module.
   - Panic on cycle detection.
5. Assign global version numbers in the final sorted order.

---

## Dependency Graph

```
bootstrap (P=0)
    └── tenant (P=10)
            └── iam (P=20)
                    └── finance (P=30)
```

Future modules declare their `DependsOn` and get sorted automatically.

---

## SQL File Convention

Every SQL file must follow this naming pattern:

```
{NNN}_{description}.up.sql
{NNN}_{description}.down.sql
```

- `NNN` — 3-digit zero-padded integer, 001–999.
- `description` — lowercase snake_case, describes what the step does.
- `.up.sql` — required; contains the forward migration.
- `.down.sql` — optional; contains the rollback. Omit only for irreversible steps.

---

## SQL Standards

Every migration file must follow these standards:

### Transaction boundaries

golang-migrate wraps each step in a transaction automatically. Do not add
`BEGIN`/`COMMIT` inside `.sql` files unless you need explicit savepoints.

### CREATE IF NOT EXISTS

Always use `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, etc.
This makes re-running a migration after partial failure safe.

### RLS on every tenant-scoped table

```sql
ALTER TABLE my_table ENABLE ROW LEVEL SECURITY;
ALTER TABLE my_table FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON my_table
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());
```

`FORCE ROW LEVEL SECURITY` ensures the table owner (superuser) is also subject to RLS.

### updated_at trigger

```sql
CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON my_table
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

Omit only on append-only or immutable tables (no `updated_at` column).

### Down migration order

Drop in reverse dependency order: indexes → triggers → policies → table.

```sql
DROP TRIGGER IF EXISTS trg_set_updated_at ON my_table;
DROP POLICY  IF EXISTS tenant_isolation   ON my_table;
DROP INDEX   IF EXISTS idx_my_table_col;
DROP TABLE   IF EXISTS my_table;
```

### Comments

Add `COMMENT ON TABLE` and `COMMENT ON COLUMN` for any non-obvious design decision.

---

## Running Migrations

### Embedded mode (production)

```bash
migrate -mode=embedded -db $DATABASE_URL up
```

Uses all registered module sources. No SQL directory needed.

### File mode (development override)

```bash
migrate -mode=file -dir ./db/migration -db $DATABASE_URL up
```

Falls back to the traditional file-based approach.

### Other commands

```bash
migrate -db $DATABASE_URL version           # current version
migrate -db $DATABASE_URL down 1           # roll back 1 step
migrate -db $DATABASE_URL force 20003      # force version (dirty recovery)
```

---

## Adding a New Module

1. Create `your_module/migrations/` directory.
2. Add SQL files: `001_create_foo.up.sql`, `001_create_foo.down.sql`, etc.
3. Add `migrations.go` with:
   ```go
   //go:embed *.sql
   var sqlFS embed.FS

   func init() {
       migration.Register(migration.Source{
           Module:    "your_module",
           Priority:  40,           // pick a tier not in use
           DependsOn: []string{"bootstrap", "tenant"},
           FS:        sqlFS,
       })
   }
   ```
4. Add blank import to `awo/cmd/migrate/main.go`:
   ```go
   _ "awo.so/your_module/migrations"
   ```

---

## Known Limitations

- **Max 999 local steps per module.** Sufficient for any foreseeable module.
- **Priority collision**: two modules with the same Priority and overlapping local
  step numbers produce a panic at startup. Assign Priority tiers with gaps.
- **No partial rollback across modules.** Rolling back `iam` step 3 does not roll
  back `finance` (which depends on iam). Always roll back the highest module first.
- **Immutable version numbers.** Once a version is applied to any environment,
  the `(module, step, global_version)` mapping must never change.
