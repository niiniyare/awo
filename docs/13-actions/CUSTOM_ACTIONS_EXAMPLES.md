# Custom Actions — Finance Module Examples

**Classification:** Reference — Tier 2
**Owner:** `13-actions/CUSTOM_ACTIONS_EXAMPLES.md`
**Status:** Living document

---

## Purpose

Canonical worked examples of custom action handlers from the Finance module. Use these as templates for new actions in any module.

---

## 1. Invoice Submit Action

Route: `POST /api/v1/entities/finance_invoice/{id}/submit`

```go
func SubmitInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    repo := action.Runtime.Repo("finance_invoice")

    rec, err := repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, err
    }

    if rec.GetString("status") != "Draft" {
        return nil, &def.BusinessError{
            Code:    "invoice.not_draft",
            Message: "Only Draft invoices can be submitted.",
            Status:  409,
        }
    }

    err = action.Runtime.Tx(ctx, func(ctx context.Context) error {
        _, err := repo.Update(ctx, action.RecordID, map[string]any{
            "status":       "Submitted",
            "submitted_by": action.Runtime.Actor().UserID,
            "submitted_at": action.Runtime.Clock(),
        })
        if err != nil {
            return fmt.Errorf("SubmitInvoiceAction: update: %w", err)
        }
        return action.Runtime.Publish(ctx, def.ActionEvent{
            Topic: "finance.invoice.submitted",
            Payload: map[string]any{
                "invoice_id":   action.RecordID,
                "submitted_by": action.Runtime.Actor().UserID,
                "total":        rec.GetDecimal("total"),
                "tenant_id":    action.Runtime.TenantID(),
                "event_at":     action.Runtime.Clock(),
            },
        })
    })
    if err != nil {
        return nil, err
    }

    workflowID, err := action.Runtime.StartWorkflow(ctx, def.ActionWorkflowSpec{
        WorkflowFn: "InvoiceApprovalWorkflow",
        TaskQueue:  "finance.invoice.approval",
        Input: InvoiceApprovalInput{
            TenantID:  action.Runtime.TenantID(),
            InvoiceID: action.RecordID,
        },
    })
    if err != nil {
        action.Runtime.Logger().Error("invoice.submit: start workflow failed",
            "err", err, "invoice_id", action.RecordID)
        // Entity is saved — workflow failure does not roll back the submit
    }

    return &def.ActionResult{
        Message:    "Invoice submitted for approval.",
        WorkflowID: workflowID,
    }, nil
}
```

---

## 2. Invoice Approve Action

Route: `POST /api/v1/entities/finance_invoice/{id}/approve`

```go
func ApproveInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    repo := action.Runtime.Repo("finance_invoice")

    rec, err := repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, err
    }

    if rec.GetString("status") != "Submitted" {
        return nil, &def.BusinessError{
            Code:    "invoice.not_submitted",
            Message: "Only Submitted invoices can be approved.",
            Status:  409,
        }
    }

    // Approver cannot be the submitter (four-eyes principle)
    submittedBy, _ := rec.GetUUID("submitted_by")
    if submittedBy == action.Runtime.Actor().UserID {
        return nil, &def.BusinessError{
            Code:    "invoice.self_approval",
            Message: "The submitter cannot approve their own invoice.",
            Status:  403,
        }
    }

    err = action.Runtime.Tx(ctx, func(ctx context.Context) error {
        _, err := repo.Update(ctx, action.RecordID, map[string]any{
            "status":      "Approved",
            "approved_by": action.Runtime.Actor().UserID,
            "approved_at": action.Runtime.Clock(),
        })
        if err != nil {
            return fmt.Errorf("ApproveInvoiceAction: update: %w", err)
        }
        return action.Runtime.Publish(ctx, def.ActionEvent{
            Topic: "finance.invoice.approved",
            Payload: map[string]any{
                "invoice_id":  action.RecordID,
                "approved_by": action.Runtime.Actor().UserID,
                "approved_at": action.Runtime.Clock(),
                "tenant_id":   action.Runtime.TenantID(),
            },
        })
    })
    if err != nil {
        return nil, err
    }

    return &def.ActionResult{
        Message: "Invoice approved.",
    }, nil
}
```

---

## 3. Invoice Cancel Action (With Reason)

Route: `POST /api/v1/entities/finance_invoice/{id}/cancel`

```go
func CancelInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    reason, ok := action.Body["reason"].(string)
    if !ok || strings.TrimSpace(reason) == "" {
        return nil, &def.ValidationError{
            Fields: map[string]string{
                "reason": "Cancellation reason is required.",
            },
        }
    }

    repo := action.Runtime.Repo("finance_invoice")
    rec, err := repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, err
    }

    status := rec.GetString("status")
    if status == "Cancelled" {
        return nil, &def.BusinessError{
            Code:    "invoice.already_cancelled",
            Message: "Invoice is already cancelled.",
            Status:  409,
        }
    }
    if status == "Paid" {
        return nil, &def.BusinessError{
            Code:    "invoice.paid_no_cancel",
            Message: "Paid invoices cannot be cancelled. Issue a credit note instead.",
            Status:  409,
        }
    }

    err = action.Runtime.Tx(ctx, func(ctx context.Context) error {
        _, err := repo.Update(ctx, action.RecordID, map[string]any{
            "status":       "Cancelled",
            "cancelled_by": action.Runtime.Actor().UserID,
            "cancelled_at": action.Runtime.Clock(),
            "cancel_reason": reason,
        })
        if err != nil {
            return fmt.Errorf("CancelInvoiceAction: update: %w", err)
        }
        return action.Runtime.Publish(ctx, def.ActionEvent{
            Topic: "finance.invoice.cancelled",
            Payload: map[string]any{
                "invoice_id":   action.RecordID,
                "cancelled_by": action.Runtime.Actor().UserID,
                "reason":       reason,
                "tenant_id":    action.Runtime.TenantID(),
            },
        })
    })
    if err != nil {
        return nil, err
    }

    return &def.ActionResult{
        Message:    "Invoice cancelled.",
        RedirectTo: "../",  // navigate back to list
    }, nil
}
```

---

## 4. Payment Record Action (With Body Validation)

Route: `POST /api/v1/entities/finance_invoice/{id}/record_payment`

```go
func RecordPaymentAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    // Parse and validate body
    amountRaw, ok := action.Body["amount"]
    if !ok {
        return nil, &def.ValidationError{
            Fields: map[string]string{"amount": "Payment amount is required."},
        }
    }
    amount, ok := amountRaw.(float64)
    if !ok || amount <= 0 {
        return nil, &def.ValidationError{
            Fields: map[string]string{"amount": "Payment amount must be a positive number."},
        }
    }

    method, _ := action.Body["method"].(string)
    if method == "" {
        return nil, &def.ValidationError{
            Fields: map[string]string{"method": "Payment method is required."},
        }
    }

    invoiceRepo := action.Runtime.Repo("finance_invoice")
    paymentRepo := action.Runtime.Repo("finance_payment")

    invoice, err := invoiceRepo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, err
    }

    if invoice.GetString("status") != "Approved" {
        return nil, &def.BusinessError{
            Code:    "invoice.not_approved",
            Message: "Payments can only be recorded against Approved invoices.",
            Status:  409,
        }
    }

    var paymentID uuid.UUID
    err = action.Runtime.Tx(ctx, func(ctx context.Context) error {
        payment, err := paymentRepo.Create(ctx, map[string]any{
            "invoice_id": action.RecordID,
            "amount":     amount,
            "method":     method,
            "paid_at":    action.Runtime.Clock(),
            "recorded_by": action.Runtime.Actor().UserID,
        })
        if err != nil {
            return fmt.Errorf("RecordPaymentAction: create payment: %w", err)
        }
        paymentID = payment.ID

        _, err = invoiceRepo.Update(ctx, action.RecordID, map[string]any{
            "status": "Paid",
        })
        if err != nil {
            return fmt.Errorf("RecordPaymentAction: update invoice: %w", err)
        }

        return action.Runtime.Publish(ctx, def.ActionEvent{
            Topic: "finance.invoice.paid",
            Payload: map[string]any{
                "invoice_id": action.RecordID,
                "payment_id": paymentID,
                "amount":     amount,
                "method":     method,
                "tenant_id":  action.Runtime.TenantID(),
            },
        })
    })
    if err != nil {
        return nil, err
    }

    return &def.ActionResult{
        Message: "Payment recorded. Invoice marked as Paid.",
        Data: map[string]any{
            "payment_id": paymentID,
        },
    }, nil
}
```

---

## Pattern Summary

| Pattern | Use When |
|---------|----------|
| Load → validate status → update in TX → publish → start workflow | State machine transitions (submit, approve) |
| Parse body → validate → load → business rule → TX | Actions with request body (cancel, record payment) |
| Read-only computation | Report generation, PDF preview |
| Two-phase commit (TX + workflow) | Actions that trigger long-running processes |

---

## References

- [`13-actions/ACTION_HANDLER_GUIDE.md`](ACTION_HANDLER_GUIDE.md) — Complete handler authoring guide
- [`13-actions/ACTION_RUNTIME_REFERENCE.md`](ACTION_RUNTIME_REFERENCE.md) — ActionRuntime methods
- [`99-modules/FINANCE_MODULE_SPEC.md`](../99-modules/FINANCE_MODULE_SPEC.md) — Finance entity definitions
