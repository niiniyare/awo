> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Aggregations"
id: pers-006
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityRepository](entity-repository.md)"
  - "[Filter DSL](filter-dsl.md)"
  - "[Cursors and Pagination](cursors-and-pagination.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Aggregations

**PERS-006 | Status: Accepted | Stability: Stable**

This document specifies the aggregate and count operations on `EntityRepository`: supported aggregate functions, grouping, and dashboard use patterns.

---

## 1. Count

```go
total, err := repo.Count(ctx, filter.Eq("status", "Submitted"))
// SELECT COUNT(*) FROM finance_invoice WHERE tenant_id = ... AND status = 'Submitted'
```

`Count` always uses the current tenant context (RLS). PolicyFunc row-level filters apply.

---

## 2. Aggregate

```go
result, err := repo.Aggregate(ctx, filter.All(), entity.AggregateSpec{
    Functions: []entity.AggregateFunc{
        {Op: entity.Sum,  Field: "total_kes", Alias: "revenue"},
        {Op: entity.Avg,  Field: "total_kes", Alias: "avg_invoice"},
        {Op: entity.Max,  Field: "total_kes", Alias: "largest"},
        {Op: entity.Min,  Field: "total_kes", Alias: "smallest"},
        {Op: entity.Count, Field: "*",        Alias: "total_count"},
    },
})

// result.Values["revenue"]     → decimal.Decimal
// result.Values["avg_invoice"] → decimal.Decimal
// result.Values["total_count"] → int64
```

### Supported Aggregate Operations

| Operation | Applicable Field Types |
|---|---|
| `Sum` | `Int`, `Currency`, `Float` |
| `Avg` | `Int`, `Currency`, `Float` |
| `Max` | `Int`, `Currency`, `Float`, `Date`, `DateTime` |
| `Min` | `Int`, `Currency`, `Float`, `Date`, `DateTime` |
| `Count` | Any (including `*`) |
| `CountDistinct` | Any scalar |

---

## 3. Grouped Aggregations

```go
result, err := repo.Aggregate(ctx, filter.Gte("created_at", startOfYear), entity.AggregateSpec{
    GroupBy: []string{"status"},
    Functions: []entity.AggregateFunc{
        {Op: entity.Count, Field: "*",        Alias: "count"},
        {Op: entity.Sum,   Field: "total_kes", Alias: "total"},
    },
})

// result.Groups = []map[string]any{
//   {"status": "Draft",     "count": 12,  "total": 45000.00},
//   {"status": "Submitted", "count": 87,  "total": 432000.00},
//   {"status": "Paid",      "count": 203, "total": 1234567.00},
// }
```

GroupBy fields must be exact matches (not computed). Grouping by JSONB custom fields requires a GIN index on the field.

---

## 4. Time-Series Aggregation

For trend charts and period reports:

```go
result, err := repo.Aggregate(ctx,
    filter.And(
        filter.Gte("created_at", sixMonthsAgo),
        filter.Eq("status", "Paid"),
    ),
    entity.AggregateSpec{
        TimeSeries: &entity.TimeSeriesSpec{
            Field:    "created_at",
            Interval: entity.IntervalMonth,
            TimeZone: "Africa/Nairobi",
        },
        Functions: []entity.AggregateFunc{
            {Op: entity.Sum,   Field: "total_kes", Alias: "revenue"},
            {Op: entity.Count, Field: "*",          Alias: "count"},
        },
    },
)

// result.Groups = []map[string]any{
//   {"period": "2024-01", "revenue": 234000.00, "count": 45},
//   {"period": "2024-02", "revenue": 198000.00, "count": 38},
//   ...
// }
```

`TimeZone` is required for monthly/weekly grouping — the period boundary depends on the tenant's timezone. Use `tenant.TimezoneFromContext(ctx)` to get the tenant's configured timezone.

### Supported Intervals

| Interval | SQL Equivalent |
|---|---|
| `IntervalDay` | `DATE_TRUNC('day', field AT TIME ZONE tz)` |
| `IntervalWeek` | `DATE_TRUNC('week', field AT TIME ZONE tz)` |
| `IntervalMonth` | `DATE_TRUNC('month', field AT TIME ZONE tz)` |
| `IntervalQuarter` | `DATE_TRUNC('quarter', field AT TIME ZONE tz)` |
| `IntervalYear` | `DATE_TRUNC('year', field AT TIME ZONE tz)` |

---

## 5. Exists Check

For existence checks without loading the record:

```go
exists, err := repo.Exists(ctx, filter.Eq("email", "alice@example.com"))
// SELECT EXISTS(SELECT 1 FROM ... WHERE email = $1 AND tenant_id = ...)
```

`Exists` is more efficient than `Count` when only a boolean is needed — it short-circuits on the first matching row.

Use `Exists` in `before_validate` hooks to check uniqueness without loading the full record:

```go
func (h *EmailUniqueGuard) BeforeValidate(ctx context.Context, record *entity.EntityRecord) error {
    email := record.Fields["email"].(string)

    // Exclude current record on update (record.ID is set for updates, zero for creates)
    f := filter.Eq("email", email)
    if record.ID != uuid.Nil {
        f = filter.And(f, filter.Ne("id", record.ID))
    }

    exists, err := h.Repo.Exists(ctx, f)
    if err != nil { return err }
    if exists {
        return &errors.ValidationError{Fields: map[string]string{"email": "Email address is already in use."}}
    }
    return nil
}
```

---

## 6. AggregateResult Type

```go
type AggregateResult struct {
    // For un-grouped aggregates
    Values map[string]any

    // For grouped aggregates (GroupBy or TimeSeries)
    Groups []map[string]any
}
```

Numeric results are returned as `decimal.Decimal` for `Currency` fields and `float64` for `Float` fields. Count results are `int64`. Date/DateTime results are `time.Time`.

---

## 7. Performance Notes

- `Count` without filter does `COUNT(*)` — fast with index
- `Aggregate` with `GroupBy` requires a full scan of matching rows — ensure the filter fields have indexes
- `TimeSeries` with `DATE_TRUNC` prevents index range scans on the time field — consider a partial index or materialized view for frequently-queried ranges
- For dashboard KPIs refreshed every page load, consider caching aggregate results in Redis (30-second TTL) — the framework does not cache aggregate results automatically

---

## Related Documents

- [EntityRepository](entity-repository.md) — full interface specification
- [Filter DSL](filter-dsl.md) — filter predicates applied before aggregation
- [Cursors and Pagination](cursors-and-pagination.md) — for list queries (not aggregations)
- [Dashboard Patterns](../08-sdui/dashboard-patterns.md) — consuming aggregate results in SDUI
