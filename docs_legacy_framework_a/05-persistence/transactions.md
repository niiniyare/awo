> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Transactions"
id: pers-007
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityRepository](entity-repository.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Sagas](../09-workflow/sagas.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Transactions

**PERS-007 | Status: Accepted | Stability: Stable**

This document specifies transaction semantics in Awo: the `WithTx` method, transaction propagation through hooks, savepoints, and the boundary with Temporal sagas.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. WithTx

All multi-step writes that must be atomic use `WithTx`:

```go
err := repo.WithTx(ctx, func(ctx context.Context, txRepo entity.EntityRepository[Invoice]) error {
    // All operations on txRepo use the same PostgreSQL transaction

    invoice, err := txRepo.Create(ctx, entity.CreateInput{...})
    if err != nil {
        return err  // transaction rolls back
    }

    for _, line := range lines {
        _, err := lineRepo.Create(ctx, entity.CreateInput{...})
        // lineRepo must be the tx-scoped version — see §2
        if err != nil {
            return err  // transaction rolls back
        }
    }

    return nil  // transaction commits
})
```

`WithTx` behavior:
- Begins a PostgreSQL transaction
- Calls `fn` with a transaction-scoped repository
- On `fn` returning nil: commits
- On `fn` returning non-nil error: rolls back
- Panics: roll back + re-panic (caller's panic handler takes over)

---

## 2. Multi-Repository Transactions

When a transaction spans multiple entity types, all repositories MUST use the same transaction. Use the `WithTxContext` pattern:

```go
// The transaction context carries the *pgx.Tx
txCtx, commit, rollback, err := store.BeginTx(ctx)
if err != nil {
    return err
}
defer rollback()

invoice, err := invoiceRepo.CreateInContext(txCtx, entity.CreateInput{...})
if err != nil { return err }

_, err = lineRepo.CreateInContext(txCtx, entity.CreateInput{...})
if err != nil { return err }

_, err = ledgerRepo.CreateInContext(txCtx, entity.CreateInput{...})
if err != nil { return err }

return commit()
```

Or use `WithTx` on one repository and pass the resulting `ctx` to other repositories:

```go
err := invoiceRepo.WithTx(ctx, func(ctx context.Context, txInvoiceRepo entity.EntityRepository[Invoice]) error {
    invoice, err := txInvoiceRepo.Create(ctx, entity.CreateInput{...})
    if err != nil { return err }

    // Pass tx-carrying ctx to other repos — they join the same transaction
    _, err = lineRepo.Create(ctx, entity.CreateInput{...})
    return err
})
```

The `ctx` passed to `lineRepo.Create` contains the transaction handle. The framework's store layer detects this and joins the existing transaction rather than beginning a new one.

---

## 3. Transactions in Hooks

`after_save` hooks run inside the entity's transaction:

```go
// HookSet
AfterCreate: []entity.AfterCreateHook{&JournalEntryCreator{}},
```

```go
func (h *JournalEntryCreator) AfterCreate(ctx context.Context, record *entity.EntityRecord) error {
    // ctx carries the transaction — ledger entry is part of the same TX as the invoice
    _, err := h.LedgerRepo.Create(ctx, entity.CreateInput{...})
    if err != nil {
        return err  // causes the entire TX (including the invoice) to roll back
    }
    return nil
}
```

If `AfterCreate` returns an error, the entire transaction rolls back — the parent entity is not persisted. Use this for writes that MUST be atomic with the parent entity (e.g., double-entry ledger lines).

`before_validate` and `before_save` hooks run outside the transaction. They MUST NOT write to the database (or if they do, it is not part of the entity's transaction).

---

## 4. Savepoints (Nested Transactions)

PostgreSQL savepoints are available for partial rollback within a transaction:

```go
err := repo.WithTx(ctx, func(ctx context.Context, txRepo entity.EntityRepository[Invoice]) error {
    // Main work
    invoice, err := txRepo.Create(ctx, entity.CreateInput{...})
    if err != nil { return err }

    // Optional: try to send email notification — rollback only this if it fails
    sp, err := store.Savepoint(ctx, "email_notification")
    if err != nil { return err }

    err = h.EmailRepo.Create(ctx, entity.CreateInput{...})
    if err != nil {
        // Roll back only the email record; keep the invoice
        if rbErr := sp.Rollback(ctx); rbErr != nil {
            return rbErr
        }
        // Log and continue without email
        slog.Warn("email notification failed, skipping", "err", err)
        return nil
    }

    return sp.Release(ctx)
})
```

Savepoints are rarely needed. Use them only when a sub-operation should fail gracefully without aborting the primary write.

---

## 5. Transaction Isolation

Awo uses PostgreSQL's default isolation level: **Read Committed**.

For operations that require serializable isolation (e.g., preventing double-spend on a resource):

```go
err := repo.WithTxIsolation(ctx, entity.Serializable, func(ctx context.Context, txRepo entity.EntityRepository[Invoice]) error {
    // SELECT ... FOR UPDATE within serializable TX
    invoice, err := txRepo.Get(ctx, invoiceID, entity.ForUpdate())
    if err != nil { return err }

    if invoice.Status != "Draft" {
        return &errors.BusinessError{Code: "invoice.not_draft", Status: 409}
    }

    _, err = txRepo.Update(ctx, invoiceID, entity.UpdateInput{
        Fields: map[string]any{"status": "Submitted"},
    })
    return err
})
```

`entity.ForUpdate()` adds `SELECT ... FOR UPDATE` — locks the row until the transaction completes, preventing concurrent modifications.

Use serializable isolation and `FOR UPDATE` only when race conditions would cause correctness problems. The additional locking reduces concurrency.

---

## 6. Transaction vs Saga

| Use case | Mechanism |
|---|---|
| Multiple entities in same DB, must be atomic | `WithTx` |
| After-save side effect atomic with parent | `after_save` hook (inside TX) |
| Cross-service write (DB + external API) | Temporal Saga |
| Long-running process (hours/days) | Temporal Workflow |
| Multiple distributed services | Temporal Saga |

Temporal workflows run **outside** PostgreSQL transactions. The outbox pattern bridges the gap: the outbox entry is written in the same TX as the entity; the workflow is started asynchronously by the relay.

---

## 7. Anti-Patterns

### Long-Held Transactions

```go
// WRONG: transaction held open during external API call
repo.WithTx(ctx, func(ctx context.Context, txRepo entity.EntityRepository[Invoice]) error {
    invoice, _ := txRepo.Create(ctx, ...)
    time.Sleep(5 * time.Second)  // or: external API call
    // Transaction blocks DB row for 5 seconds — connection contention
    return nil
})

// CORRECT: commit first, then call external API in a Temporal activity
```

### Nested WithTx

```go
// WRONG: nested WithTx creates a savepoint, not a new transaction
// This may not behave as expected if the inner TX semantics differ
outerRepo.WithTx(ctx, func(ctx context.Context, txRepo entity.EntityRepository[Invoice]) error {
    return innerRepo.WithTx(ctx, func(ctx context.Context, ...) error { ... })
})

// CORRECT: pass ctx from outer WithTx to inner repo directly
// The inner repo joins the outer transaction automatically
```

---

## Related Documents

- [EntityRepository](entity-repository.md) — `WithTx`, `Exists`, `Count` interface specification
- [Hooks](../04-domain/hooks.md) — which hooks run inside vs outside the transaction
- [Sagas](../09-workflow/sagas.md) — cross-service atomicity via compensation
- [Outbox Pattern](../09-workflow/outbox-pattern.md) — bridging DB TX to async workflow start
