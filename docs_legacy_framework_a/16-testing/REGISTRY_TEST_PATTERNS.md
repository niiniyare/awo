> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Registry Test Patterns

**Classification:** Reference — Tier 2
**Owner:** `16-testing/REGISTRY_TEST_PATTERNS.md`
**Status:** Living document

---

## Purpose

Patterns for testing entity registration, compiler output, and validation rules using isolated registry instances.

---

## 1. Isolated Registry Pattern

Use `registry.BuildFrom()` to create a test registry with only the entities under test — never test against the global registry (which includes all production entities):

```go
func TestInvoiceDefinitionCompiles(t *testing.T) {
    reg := registry.BuildFrom([]def.EntityDefinition{
        &InvoiceDefinition,
        &CustomerDefinition,  // required because Invoice links to Customer
    })

    schema, diags := compiler.Compile(reg)
    if len(diags.Errors()) > 0 {
        t.Fatalf("compilation errors: %v", diags.Errors())
    }

    entitySchema, ok := schema.ByName["finance_invoice"]
    if !ok {
        t.Fatal("finance_invoice not in compiled schema")
    }
    if entitySchema.RoutePrefix != "/api/v1/entities/finance-invoice" {
        t.Errorf("wrong route prefix: %s", entitySchema.RoutePrefix)
    }
}
```

---

## 2. Testing Validation Rules

```go
func TestRegistryValidation_DuplicateName(t *testing.T) {
    def1 := &def.SystemDefinition{Name: "duplicate", Module: "test"}
    def2 := &def.SystemDefinition{Name: "duplicate", Module: "test"}

    _, err := registry.BuildFrom([]def.EntityDefinition{def1, def2})
    if err == nil {
        t.Fatal("expected error for duplicate entity name")
    }
    if !strings.Contains(err.Error(), "duplicate") {
        t.Errorf("error should mention 'duplicate', got: %v", err)
    }
}

func TestRegistryValidation_UnresolvedLink(t *testing.T) {
    def1 := &def.SystemDefinition{
        Name:   "invoice",
        Module: "test",
        Fields: []def.FieldDef{
            {Name: "customer_id", Type: def.FieldTypeLink, LinkTarget: "customer"},
        },
    }
    // customer definition not registered

    _, diags := compiler.Compile(registry.BuildFrom([]def.EntityDefinition{def1}))
    if !diags.HasErrors() {
        t.Fatal("expected compiler error for unresolved link target")
    }
}
```

---

## 3. Testing CapabilityGrant Output

```go
func TestInvoicePermissions_GrantedToCorrectRoles(t *testing.T) {
    reg := registry.BuildFrom([]def.EntityDefinition{&InvoiceDefinition})
    schema, _ := compiler.Compile(reg)

    grants := schema.CapabilityGrants
    var createGrants []string
    for _, g := range grants {
        if g.Entity == "finance_invoice" && g.Action == "create" {
            createGrants = append(createGrants, g.Role)
        }
    }

    want := []string{"role:finance.accounts_payable", "role:tenant.admin"}
    if !stringSlicesMatch(createGrants, want) {
        t.Errorf("create grants = %v, want %v", createGrants, want)
    }
}
```

---

## 4. Testing Route Generation

```go
func TestInvoiceRoutes_GeneratedCorrectly(t *testing.T) {
    reg := registry.BuildFrom([]def.EntityDefinition{&InvoiceDefinition})
    schema, _ := compiler.Compile(reg)

    routeMap := make(map[string]bool)
    for _, r := range schema.Routes {
        routeMap[r.Method+":"+r.Path] = true
    }

    required := []string{
        "GET:/api/v1/entities/finance-invoice",
        "GET:/api/v1/entities/finance-invoice/:id",
        "POST:/api/v1/entities/finance-invoice",
        "PATCH:/api/v1/entities/finance-invoice/:id",
        "DELETE:/api/v1/entities/finance-invoice/:id",
        "POST:/api/v1/entities/finance-invoice/:id/submit",
        "POST:/api/v1/entities/finance-invoice/:id/approve",
    }

    for _, route := range required {
        if !routeMap[route] {
            t.Errorf("missing route: %s", route)
        }
    }
}
```

---

## 5. TestMain for Schema Tests

When testing an entire module's compilation:

```go
var testSchema *compiler.CompiledSchema

func TestMain(m *testing.M) {
    reg := registry.BuildFrom(financeModule.AllDefinitions())
    var diags compiler.Diagnostics
    testSchema, diags = compiler.Compile(reg)
    if diags.HasErrors() {
        fmt.Fprintf(os.Stderr, "compilation failed: %v\n", diags.Errors())
        os.Exit(1)
    }
    os.Exit(m.Run())
}

func TestFinanceModule_AllEntitiesPresent(t *testing.T) {
    required := []string{
        "finance_invoice",
        "finance_invoice_line",
        "finance_customer",
        "finance_payment",
        "finance_journal_entry",
    }
    for _, name := range required {
        if _, ok := testSchema.ByName[name]; !ok {
            t.Errorf("missing entity: %s", name)
        }
    }
}
```

---

## References

- [`16-testing/TEST_STRATEGY.md`](TEST_STRATEGY.md) — Test strategy overview
- [`05-compiler/COMPILE_SPEC.md`](../05-compiler/COMPILE_SPEC.md) — Compiler phases
- [`05-compiler/VALIDATION_RULES.md`](../05-compiler/VALIDATION_RULES.md) — Validation rules tested here
