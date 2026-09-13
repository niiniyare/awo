# Sensitive Fields

**Classification:** Specification — Tier 1
**Owner:** `18-security/SENSITIVE_FIELDS.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the semantics of `Sensitive: true` on `FieldDef` — what is excluded, where, and why.

---

## 1. Declaration

```go
def.Field("national_id").OfType(def.FieldTypeData).Sensitive()
// or in struct literal:
def.FieldDef{Name: "bank_account", Type: def.FieldTypeData, Sensitive: true}
```

---

## 2. What Sensitive: true Does

| Location | Behaviour |
|----------|-----------|
| **List view (SDUI)** | Field is ABSENT from auto-generated list columns |
| **Detail view (SDUI)** | Field rendered as `password`-type input — value masked |
| **Audit records** | Field is excluded from `before_data` and `after_data` |
| **Log entries** | Field is excluded from `EntityRecord.ToLogMap()` output |
| **API responses (list)** | Field is ABSENT from list endpoint responses |
| **API responses (single)** | Field is included — explicit single-record read grants access |
| **Filter DSL** | `Searchable: true` AND `Sensitive: true` is prohibited (would leak via search) |

---

## 3. What Sensitive: true Does NOT Do

- Does NOT prevent the field from being returned on single-record GET (`GET /:id`).
- Does NOT encrypt the field at rest — that requires column-level encryption, which is out of scope for v1.
- Does NOT prevent the field from being written via `PATCH` (if the actor has write permission).
- Does NOT prevent the field from appearing in database queries (RLS protects at the row level; sensitive protection is at the field level in application code only).

---

## 4. Fields That MUST Be Marked Sensitive

The following field types MUST always be declared `Sensitive: true`:

| Field | Entity | Why |
|-------|--------|-----|
| `national_id` | Person, Employee | PII — Kenyan national ID number |
| `passport_number` | Person | PII |
| `bank_account` | any | Financial PII |
| `kra_pin` | Person, Company | Tax ID — sensitive by regulation |
| `mpesa_number` | any | Financial channel |
| `salary` | Employee | Compensation confidentiality |
| `password_hash` | User | Security credential |
| `api_key` | Integration | Security credential |
| `webhook_secret` | Integration | Security credential |

---

## 5. Sensitive Fields in Custom Fields (JSONB)

Custom fields added via the Metadata module inherit their sensitivity from the `CustomFieldDef` declaration. The `CustomFieldDef` has a `Sensitive bool` field identical to `FieldDef.Sensitive`.

For system entities with `custom_fields jsonb`, the entire JSONB blob is included in audit records. Custom field sensitivity filtering within the JSONB blob is applied per-key based on the `CustomFieldDef` registry at the time of the audit write.

---

## 6. SDUI Rendering Detail

In the detail view, sensitive fields are rendered as:

```json
{
  "type": "input-text",
  "name": "national_id",
  "label": "National ID",
  "inputType": "password",
  "readOnly": true
}
```

This masks the value in the browser (shown as `••••••••`) while keeping the field visible to the user who explicitly opened the record. Clipboard access to the unmasked value depends on browser behaviour.

---

## 7. Normative Requirements

- `Sensitive: true` fields MUST be absent from list view responses (both SDUI schema and API).
- `Sensitive: true` fields MUST be excluded from `audit_log.before_data` and `audit_log.after_data`.
- `Sensitive: true` fields MUST NOT appear in log entries via `ToLogMap()`.
- `Searchable: true` AND `Sensitive: true` on the same field MUST cause a compiler error.
- New PII fields MUST be reviewed against this list before merge.

---

## References

- [`01-entity/FIELD_TYPES_REFERENCE.md`](../01-entity/FIELD_TYPES_REFERENCE.md) — FieldDef.Sensitive
- [`12-audit/AUDIT_SPEC.md`](../12-audit/AUDIT_SPEC.md) — Audit field exclusion
- [`17-observability/LOGGING_SPEC.md`](../17-observability/LOGGING_SPEC.md) — Log field exclusion
- [`10-sdui/SDUI_FIELD_WIDGET_MAP.md`](../10-sdui/SDUI_FIELD_WIDGET_MAP.md) — List exclusion
