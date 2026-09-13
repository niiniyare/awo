> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Finance: KRA eTIMS Integration"
id: mod-016
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Finance Patterns](finance-patterns.md)"
  - "[Feature Flags Module](platform-flags-module.md)"
  - "[Workflow Engine](../09-workflow/workflow-engine.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Finance: KRA eTIMS Integration

**MOD-016 | Status: Accepted | Stability: Stable**

Electronic Tax Invoice Management System (eTIMS) integration with the Kenya Revenue Authority: invoice submission, TaxEntry entity, error handling, and compliance requirements.

---

## 1. What is eTIMS

KRA eTIMS is Kenya's electronic tax invoice system. VAT-registered businesses must submit tax invoices to KRA within a specified window (currently 24 hours of invoice creation). Non-compliance results in penalties.

The integration is:
- **Opt-in per tenant**: controlled by `finance.etims_integration` feature flag
- **Asynchronous**: submission happens in a Temporal workflow, not synchronously in the request
- **Idempotent**: each invoice generates exactly one eTIMS submission; retries use the same control unit number

---

## 2. TaxEntry Entity

Each KRA eTIMS submission creates a `tax_entry` system entity (not a custom entity — regulatory compliance requires SQL constraints):

```go
var TaxEntryDefinition = def.SystemDefinition{
    Name:        "finance_tax_entry",
    Module:      "finance",
    Label:       "Tax Entry",
    LabelPlural: "Tax Entries",
    Fields: []def.FieldDef{
        {Name: "invoice",         Type: def.FieldLink,     LinkTarget: "finance_invoice", Required: true, Immutable: true},
        {Name: "invoice_number",  Type: def.FieldData,     Required: true, Immutable: true},
        {Name: "customer_pin",    Type: def.FieldData,     Required: true, Immutable: true},  // KRA PIN
        {Name: "total_kes",       Type: def.FieldCurrency, Required: true, Immutable: true},
        {Name: "vat_kes",         Type: def.FieldCurrency, Required: true, Immutable: true},
        {Name: "status",          Type: def.FieldSelect, Required: true,
            Options: []string{"Pending", "Submitted", "Accepted", "Rejected", "Error"},
            Default: "Pending"},
        {Name: "control_unit_number", Type: def.FieldData},  // assigned by KRA on acceptance
        {Name: "submitted_at",    Type: def.FieldDateTime},
        {Name: "kra_response",    Type: def.FieldJSON},      // raw KRA response
        {Name: "error_message",   Type: def.FieldSmallText},
        {Name: "retry_count",     Type: def.FieldInt, Default: 0},
    },
    Permissions: def.PermissionSet{
        Read:   []string{"role:finance.viewer", "role:tenant.admin"},
        Create: []string{"role:system.internal_only"}, // only created by workflow
        Write:  []string{"role:system.internal_only"},
        Delete: []string{}, // immutable — regulatory requirement
    },
}
```

`TaxEntry` records are never deleted — KRA requires a 5-year audit trail.

---

## 3. Workflow Integration

eTIMS submission is triggered after invoice approval. It is not in the main transaction:

```go
// finance/def.go
WorkflowTriggers: []def.WorkflowTrigger{
    {
        On:         def.EventOnSubmit,   // invoice status → Submitted
        WorkflowFn: "InvoiceETIMSWorkflow",
        TaskQueue:  "finance.etims",
        Condition: func(rec *def.EntityRecord) bool {
            // Only trigger if eTIMS flag is enabled
            return flags.IsEnabled(rec.TenantID, "finance.etims_integration")
        },
        InputBuilder: func(rec *def.EntityRecord, tc def.TriggerContext) (any, error) {
            return finance.ETIMSInput{
                TenantID:  rec.TenantID,
                InvoiceID: rec.ID,
            }, nil
        },
    },
},
```

---

## 4. ETIMSWorkflow

```go
func InvoiceETIMSWorkflow(ctx workflow.Context, input ETIMSInput) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 5 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    5 * time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    1 * time.Hour,
            MaximumAttempts:    10,
            // KRA service errors are retryable; validation errors are not
            NonRetryableErrorTypes: []string{
                "etims.invalid_pin",
                "etims.duplicate_invoice_number",
            },
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // 1. Create TaxEntry record (status: Pending)
    var taxEntryID uuid.UUID
    if err := workflow.ExecuteActivity(ctx, a.CreateTaxEntryActivity, input).Get(ctx, &taxEntryID); err != nil {
        return err
    }

    // 2. Submit to KRA eTIMS API
    var response ETIMSResponse
    if err := workflow.ExecuteActivity(ctx, a.SubmitToETIMSActivity, ETIMSSubmitInput{
        TenantID:   input.TenantID,
        TaxEntryID: taxEntryID,
    }).Get(ctx, &response); err != nil {
        // Update TaxEntry to Error status
        _ = workflow.ExecuteActivity(ctx, a.MarkTaxEntryErrorActivity, taxEntryID).Get(ctx, nil)
        return err
    }

    // 3. Update TaxEntry with KRA response (control unit number)
    return workflow.ExecuteActivity(ctx, a.CompleteETIMSSubmissionActivity, CompleteInput{
        TaxEntryID:        taxEntryID,
        ControlUnitNumber: response.ControlUnitNumber,
        KRAResponse:       response.Raw,
    }).Get(ctx, nil)
}
```

---

## 5. SubmitToETIMSActivity

```go
func (a *ETIMSActivities) SubmitToETIMSActivity(ctx context.Context, input ETIMSSubmitInput) (ETIMSResponse, error) {
    // Load tax entry and invoice details
    tenantCtx, _ := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    taxEntry, err := a.TaxEntryRepo.Get(tenantCtx, input.TaxEntryID)
    if err != nil {
        return ETIMSResponse{}, fmt.Errorf("SubmitToETIMSActivity: load tax entry: %w", err)
    }

    // Load KRA credentials from tenant settings
    craPin, _ := a.Settings.Get(tenantCtx, "finance.kra_pin")
    apiKey, _ := a.Settings.GetSensitive(tenantCtx, "finance.etims_api_key")

    // Build eTIMS payload per KRA API spec
    payload := buildETIMSPayload(taxEntry, kraPin)

    // Submit to KRA
    resp, err := a.ETIMSClient.Submit(ctx, apiKey, payload)
    if err != nil {
        // Classify error
        var etErr *ETIMSError
        if errors.As(err, &etErr) {
            if etErr.Code == "INVALID_PIN" {
                return ETIMSResponse{}, temporal.NewNonRetryableApplicationError(
                    "KRA PIN is invalid", "etims.invalid_pin", err)
            }
        }
        return ETIMSResponse{}, fmt.Errorf("SubmitToETIMSActivity: KRA API: %w", err)
    }

    // Update TaxEntry status to Submitted
    _, err = a.TaxEntryRepo.Update(tenantCtx, input.TaxEntryID, def.UpdateInput{
        Fields: map[string]any{
            "status":       "Submitted",
            "submitted_at": time.Now().UTC(),
            "kra_response": resp.Raw,
        },
    })

    return *resp, err
}
```

---

## 6. eTIMS Configuration

Per-tenant settings (set during provisioning for Enterprise tenants):

| Setting Key | Description | Example |
|---|---|---|
| `finance.kra_pin` | Tenant's KRA PIN | `P000000000X` |
| `finance.etims_api_key` | KRA eTIMS API key (sensitive) | `etims_sk_...` |
| `finance.etims_branch_id` | Branch identifier for multi-branch tenants | `00` |
| `finance.etims_device_serial` | eTIMS device serial number | `XXXXXXXX` |

Sensitive settings are stored encrypted at rest. They are never logged or included in error responses.

---

## 7. Compliance Requirements

| Requirement | Implementation |
|---|---|
| Submit within 24 hours | Workflow starts immediately on invoice submit |
| Retry on KRA service failure | Temporal retry policy with exponential backoff |
| Store KRA response | `kra_response` JSON field on TaxEntry |
| 5-year audit trail | TaxEntry has no Delete permission |
| Control unit number on invoice | InvoiceDefinition has `etims_control_unit` field, populated on acceptance |

---

## 8. eTIMS Flag Gating

The entire integration is gated by `finance.etims_integration`:

- If disabled: workflow trigger condition returns false — no eTIMS workflow starts
- If enabled after being previously disabled: historical invoices are NOT retroactively submitted
- The flag should only be enabled after KRA credentials are configured in tenant settings

---

## Related Documents

- [Finance Patterns](finance-patterns.md) — invoice workflow, journal entries
- [Feature Flags Module](platform-flags-module.md) — flag evaluation
- [Workflow Error Handling](../09-workflow/error-handling.md) — retry policy, non-retryable errors
- [Settings Patterns](settings-patterns.md) — sensitive settings storage
