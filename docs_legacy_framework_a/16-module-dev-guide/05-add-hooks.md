> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add Hooks"
id: mdg-05
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add Edges](04-add-edges.md)"
  - "[Add Policies](06-add-policies.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add Hooks

**MDG-05 | Module Developer Guide**

This document adds lifecycle hooks to `crm_contact`: an email uniqueness validator (`BeforeCreate`) and a welcome email trigger (`AfterCreate`).

---

## 1. Hook Placement

Hooks live in `hooks.go`. Each hook is a struct implementing the appropriate interface:

```go
// internal/core/crm/hooks.go
package crm
```

---

## 2. BeforeCreate Hook — Email Uniqueness

The `email` field is declared `Unique: true`, which generates a database unique constraint. But for custom entities (JSONB storage), uniqueness is enforced via a hook that queries the repository, not at the database level.

```go
// ContactEmailUniqueGuard verifies email uniqueness before creating a contact.
type ContactEmailUniqueGuard struct {
    Repo entity.EntityRepository
}

func (h *ContactEmailUniqueGuard) BeforeCreate(
    ctx context.Context,
    rec *entity.EntityRecord,
    hctx entity.HookContext,
) error {
    email, _ := rec.Fields["email"].(string)
    if email == "" {
        return nil  // Required validator handles empty case
    }

    exists, err := h.Repo.Exists(ctx, filter.And(
        filter.Eq("email", email),
    ))
    if err != nil {
        return fmt.Errorf("ContactEmailUniqueGuard.BeforeCreate: check email: %w", err)
    }
    if exists {
        return &errors.ValidationError{
            Fields: map[string]string{
                "email": "A contact with this email address already exists.",
            },
        }
    }
    return nil
}
```

---

## 3. AfterCreate Hook — Set First Contact Date

An `AfterCreate` hook that defaults `first_contact_date` to today if not set:

```go
// ContactFirstContactDateSetter sets first_contact_date on creation if not provided.
type ContactFirstContactDateSetter struct{}

func (h *ContactFirstContactDateSetter) AfterCreate(
    ctx context.Context,
    rec *entity.EntityRecord,
    hctx entity.HookContext,
) error {
    if rec.Fields["first_contact_date"] != nil {
        return nil  // Already set — don't override
    }

    today := time.Now().UTC().Format("2006-01-02")
    _, err := hctx.Repo.Update(ctx, rec.ID, entity.UpdateInput{
        Fields: map[string]any{
            "first_contact_date": today,
        },
    })
    if err != nil {
        return fmt.Errorf("ContactFirstContactDateSetter.AfterCreate: %w", err)
    }
    return nil
}
```

`AfterCreate` runs **inside the transaction**. The Update call participates in the same transaction. If it fails, the entire create is rolled back.

---

## 4. Register Hooks in ContactDefinition

```go
// internal/core/crm/def.go
var ContactDefinition = def.EntityDefinition{
    // ... Name, Module, Label, StorageModel, Fields, Edges ...

    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{
            &ContactEmailUniqueGuard{
                Repo: nil, // injected at wire time — see below
            },
        },
        AfterCreate: []def.AfterCreateHook{
            &ContactFirstContactDateSetter{},
        },
        BeforeUpdate: []def.BeforeUpdateHook{
            // Add email uniqueness check on update too:
            &ContactEmailUniqueOnUpdateGuard{Repo: nil},
        },
    },
}
```

---

## 5. Dependency Injection for Hooks

Hooks that need a repository dependency (like `ContactEmailUniqueGuard`) must have their dependency injected. The framework supports this via the hook's dependency interface:

```go
// The hook declares its dependency interface
func (h *ContactEmailUniqueGuard) InjectRepo(repo entity.EntityRepository) {
    h.Repo = repo
}
```

Or use the wire pattern in `crm.go`:

```go
func init() {
    // Create hooks with dependencies
    emailGuard := &ContactEmailUniqueGuard{}

    // Wire dependencies after registry provides the repo
    def.OnRegistryReady(func(reg *def.Registry) {
        emailGuard.Repo = reg.RepoFor("crm_contact")
    })

    ContactDefinition.Hooks.BeforeCreate = append(
        ContactDefinition.Hooks.BeforeCreate, emailGuard,
    )

    def.RegisterManifest(&Manifest)
    def.Register(&ContactDefinition)
    def.Register(&InteractionDefinition)
}
```

---

## 6. Hook Error Semantics

| Hook Stage | Error Type | Effect |
|---|---|---|
| `BeforeCreate` | `*ValidationError` | Returns HTTP 422; no record created |
| `BeforeCreate` | `*BusinessError` | Returns the error's HTTP status; no record created |
| `AfterCreate` | Any error | Transaction rolled back; record NOT created |
| `BeforeUpdate` | `*ValidationError` | Returns HTTP 422; no update applied |
| `AfterUpdate` | Any error | Transaction rolled back; update NOT applied |

`AfterCreate` and `AfterUpdate` hooks run inside the transaction — errors cause rollback. Use them for side effects that must be atomic with the main operation (e.g., updating a denormalized counter, writing to a related table).

For side effects that should NOT block the main operation (e.g., sending a notification email), use a `WorkflowTrigger` instead.

---

## 7. Testing the Hooks

```go
// internal/core/crm/crm_test.go
func TestContactEmailUniqueGuard_DuplicateEmail(t *testing.T) {
    repo := &mockRepo{
        existsResult: true,  // simulate existing contact with same email
    }
    hook := &ContactEmailUniqueGuard{Repo: repo}

    err := hook.BeforeCreate(context.Background(), &entity.EntityRecord{
        Fields: map[string]any{"email": "test@example.com"},
    }, entity.HookContext{})

    var ve *errors.ValidationError
    if !errors.As(err, &ve) {
        t.Fatalf("expected ValidationError, got %T: %v", err, err)
    }
    if ve.Fields["email"] == "" {
        t.Error("expected email field error message")
    }
}

func TestContactEmailUniqueGuard_UniqueEmail(t *testing.T) {
    repo := &mockRepo{existsResult: false}
    hook := &ContactEmailUniqueGuard{Repo: repo}

    err := hook.BeforeCreate(context.Background(), &entity.EntityRecord{
        Fields: map[string]any{"email": "new@example.com"},
    }, entity.HookContext{})

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

---

## Next: [Add Policies →](06-add-policies.md)
