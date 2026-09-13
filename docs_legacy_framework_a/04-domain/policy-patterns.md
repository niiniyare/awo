> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Policy Patterns"
id: dom-013
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Policy Functions](policies.md)"
  - "[RBAC Deep Dive](../07-iam/rbac.md)"
  - "[Multi-Branch](../06-tenancy/multi-branch.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Policy Patterns

**DOM-013 | Status: Accepted | Stability: Stable**

Common row-level policy patterns: OwnerOnly, BranchScoped, role-conditional, composite, and audit-aware policies.

---

## 1. Policy vs RBAC

| Mechanism | What it does | When applied |
|---|---|---|
| RBAC (Casbin) | Operation-level gate: can this actor CREATE invoices at all? | Before handler |
| PolicyFunc | Row-level filter: which invoices can this actor SEE/WRITE? | Inside every query |

Both are required. RBAC without policies = actor sees all tenant data. Policies without RBAC = no operation gating.

---

## 2. OwnerOnly

Most common pattern: users see only records assigned to them.

```go
// internal/core/crm/policy.go

var ContactOwnerPolicy = def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    if actor == nil {
        return filter.False()  // no actor = no rows
    }
    // Tenant admins see all; sales reps see only their own
    if actor.HasRole("role:tenant.admin") {
        return filter.True()  // no additional filtering
    }
    return filter.Eq("assigned_rep", actor.UserID)
})
```

```go
// Register on EntityDefinition
var ContactDefinition = def.SystemDefinition{
    Policy: ContactOwnerPolicy,
}
```

---

## 3. BranchScoped

Users scoped to a branch see only that branch's records:

```go
var ShiftBranchPolicy = def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    if actor.HasRole("role:tenant.admin") {
        return filter.True()
    }
    branchID := session.BranchIDFromContext(ctx)
    if branchID == uuid.Nil {
        return filter.False()  // no branch context = no shifts visible
    }
    return filter.Eq("branch", branchID)
})
```

---

## 4. Role-Conditional Policy

Different filter based on actor's role:

```go
var InvoiceViewPolicy = def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    switch {
    case actor.HasRole("role:tenant.admin"):
        return filter.True()  // see all

    case actor.HasRole("role:finance.approver"):
        // Approvers see submitted + approved invoices only (not drafts)
        return filter.In("status", []any{"Submitted", "Approved", "Paid"})

    case actor.HasRole("role:finance.accounts_payable"):
        // AP staff see everything they created OR submitted to them
        return filter.Or(
            filter.Eq("created_by", actor.UserID),
            filter.Eq("assigned_approver", actor.UserID),
        )

    default:
        return filter.False()  // no recognized role = no rows
    }
})
```

---

## 5. Composite Policy

Combine multiple policies with AND:

```go
// Both conditions must be satisfied: correct branch AND owns the record
var ShiftOwnerAndBranchPolicy = def.ComposedPolicy(
    ShiftBranchPolicy,
    ShiftOwnerPolicy,
)
```

`ComposedPolicy` ANDs all filters together. An actor that fails either policy sees no rows.

---

## 6. Time-Bounded Policy

Restrict access to records within a time window (e.g., current accounting period only):

```go
var CurrentPeriodLedgerPolicy = def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    if actor.HasRole("role:finance.auditor") {
        return filter.True()  // auditors can see all periods
    }
    // Regular users see only current fiscal year
    fiscalYearStart := currentFiscalYearStart(ctx)  // reads from tenant settings
    return filter.GtEq("entry_date", fiscalYearStart)
})
```

---

## 7. Sensitive Field Masking Policy

Policy that masks fields rather than filtering rows. Declared separately from the row filter:

```go
// SensitiveFieldMask is applied when actor lacks the sensitive data role
var PayslipSensitivePolicy = def.SensitiveFieldPolicy(
    func(ctx context.Context) bool {
        actor := session.ActorFromContext(ctx)
        // Only HR admins and the employee themselves can see full salary details
        return actor.HasRole("role:hr.admin") ||
               actor.UserID == employeeIDFromRecord(ctx)
    },
    []string{"gross_salary", "net_salary", "bank_account", "nhif_number"},
)
```

Fields listed in `SensitiveFieldPolicy` are excluded from query results for actors that fail the predicate. The row is still returned — just with sensitive fields zeroed.

---

## 8. Policy Testing

```go
func TestInvoiceViewPolicy_ApproverSeesSubmitted(t *testing.T) {
    ctx := session.WithActor(context.Background(), session.Actor{
        UserID: uuid.New(),
        Roles:  []string{"role:finance.approver"},
    })

    f := InvoiceViewPolicy(ctx)

    // Verify the filter contains status IN (Submitted, Approved, Paid)
    filterExpr, _ := json.Marshal(f)
    assert.Contains(t, string(filterExpr), "Submitted")
    assert.NotContains(t, string(filterExpr), "Draft")
}

func TestInvoiceViewPolicy_NoRoleSeesNothing(t *testing.T) {
    ctx := session.WithActor(context.Background(), session.Actor{
        Roles: []string{"role:tenant.user"},  // no finance role
    })

    f := InvoiceViewPolicy(ctx)
    assert.Equal(t, filter.False(), f)
}
```

Policy functions are pure functions — no I/O, no repository calls. Test without infrastructure.

---

## 9. Policy Anti-Patterns

### Querying Database in PolicyFunc

```go
// WRONG: policy calls repository — breaks composability and causes N+1
var BadPolicy = def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    // DO NOT query the DB in a PolicyFunc
    branchIDs, _ := branchRepo.GetBranchesForUser(ctx, actor.UserID)  // BAD
    return filter.In("branch", branchIDs)
})
```

PolicyFunc must be pure — compute the filter from context only. Branch IDs should be in the actor context (loaded at session creation time).

### Missing Nil Actor Check

```go
// WRONG: panics if middleware didn't set actor
var BadPolicy = def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    return filter.Eq("owner", actor.UserID)  // panics if actor is nil
})

// CORRECT:
var GoodPolicy = def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    if actor == nil {
        return filter.False()
    }
    return filter.Eq("owner", actor.UserID)
})
```

---

## Related Documents

- [Policy Functions](policies.md) — PolicyFunc interface specification
- [RBAC Deep Dive](../07-iam/rbac.md) — Casbin roles and permissions
- [Multi-Branch](../06-tenancy/multi-branch.md) — BranchScoped policy
- [Testing Patterns](../16-module-dev-guide/15-testing-patterns.md) — policy unit testing
