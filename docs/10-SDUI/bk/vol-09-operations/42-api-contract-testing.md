# Chapter 42 — API Contract Testing

> **Volume:** IX — Testing, CI/CD & Governance
> **Phase:** 4
> **Audience:** Backend Engineers, Frontend Engineers, Mobile Engineers
> **Prerequisites:** Chapters 25, 41

---

## Table of Contents

- [42.1 Contract Testing Overview](#421-contract-testing-overview)
- [42.2 OpenAPI Contract Enforcement](#422-openapi-contract-enforcement)
- [42.3 Consumer-Driven Contracts (Pact)](#423-consumer-driven-contracts-pact)
- [42.4 SDUI Schema Contracts](#424-sdui-schema-contracts)
- [42.5 Breaking Change Detection](#425-breaking-change-detection)
- [42.6 Backwards Compatibility Tests](#426-backwards-compatibility-tests)

---

## 42.1 Contract Testing Overview

API contract testing ensures that what the server provides matches what clients expect — and that changes to either side are detected before they reach production.

AwoERP has three categories of API contract:

1. **REST API contracts** — The OpenAPI 3.1 specification generated from the Goa DSL. Every endpoint's request and response shape is specified.
2. **SDUI schema contracts** — The JSON schema for surface definitions. Every component type, every binding expression, every action shape is specified.
3. **Consumer-driven contracts** — The web app, iOS app, Android app, and terminal client each specify exactly which API fields they consume. If the server removes or renames a field those consumers use, the contract fails.

---

## 42.2 OpenAPI Contract Enforcement

### 42.2.1 Schema Generation

The OpenAPI spec is generated automatically from the Goa DSL at build time:

```bash
# Generate OpenAPI spec from Goa DSL
goa gen github.com/awoerp/api/design

# Spec lands at: gen/http/openapi3.json
```

The generated spec is committed to the repository. A CI check verifies that the committed spec matches the one generated from the current DSL — any DSL change that is not reflected in the committed spec fails the build.

### 42.2.2 Request Validation Against Spec

The API gateway validates all incoming requests against the OpenAPI spec before routing them to handlers:

```go
// Middleware: validate request against OpenAPI spec
func OpenAPIValidationMiddleware(spec *openapi3.T) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        router, _ := openapi3filter.NewRouter().WithSwagger(spec)

        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            route, params, err := router.FindRoute(r)
            if err != nil {
                http.Error(w, "not found", 404)
                return
            }

            // Validate request
            if err := openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
                Request:    r,
                PathParams: params,
                Route:      route,
            }); err != nil {
                writeError(w, 400, "invalid_request", err.Error())
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 42.2.3 Response Validation in Tests

In integration tests, responses are validated against the spec:

```go
func validateAPIResponse(t *testing.T, spec *openapi3.T, req *http.Request, resp *http.Response, body []byte) {
    t.Helper()
    route, params, err := router.FindRoute(req)
    require.NoError(t, err)

    err = openapi3filter.ValidateResponse(req.Context(), &openapi3filter.ResponseValidationInput{
        RequestValidationInput: &openapi3filter.RequestValidationInput{
            Request:    req,
            PathParams: params,
            Route:      route,
        },
        Status: resp.StatusCode,
        Header: resp.Header,
        Body:   io.NopCloser(bytes.NewReader(body)),
    })
    assert.NoError(t, err, "response does not conform to OpenAPI spec")
}
```

---

## 42.3 Consumer-Driven Contracts (Pact)

### 42.3.1 Consumer Contract Definition (Web)

The web app defines its expectations as Pact consumer contracts:

```typescript
// pact/invoice-api.pact.ts
describe('Invoice API', () => {
    const provider = new PactV3({
        consumer: 'awoerp-web',
        provider: 'awoerp-api',
    });

    describe('GET /api/invoices', () => {
        beforeEach(() =>
            provider.addInteraction({
                states: [{ description: 'invoices exist for tenant' }],
                uponReceiving: 'a request for the invoice list',
                withRequest: {
                    method: 'GET',
                    path: '/api/v1/invoices',
                    headers: {
                        Authorization: like('Bearer token'),
                        'X-Tenant-ID': like('tenant-uuid'),
                    },
                },
                willRespondWith: {
                    status: 200,
                    body: {
                        data: eachLike({
                            id: like('inv-001'),
                            status: like('draft'),
                            amount: like(100000),
                            currency_code: like('KES'),
                            created_at: like('2024-03-15T11:32:00Z'),
                        }),
                        meta: {
                            total: like(10),
                            page: like(1),
                            page_size: like(25),
                        },
                    },
                },
            })
        );

        it('returns invoice list', async () => {
            const result = await invoiceApi.list();
            expect(result.data).toHaveLength(1);
            expect(result.data[0]).toHaveProperty('currency_code');
        });
    });
});
```

### 42.3.2 Provider Verification (Server)

The server verifies consumer contracts in CI:

```go
func TestPactProvider_InvoiceAPI(t *testing.T) {
    verifier := pact.HTTPVerifier{}
    err := verifier.VerifyProvider(t, pact.VerifyRequest{
        ProviderBaseURL:    "http://localhost:8080",
        BrokerURL:          os.Getenv("PACT_BROKER_URL"),
        Provider:           "awoerp-api",
        ProviderVersion:    buildVersion,
        PublishVerificationResults: true,
        StateHandlers: pact.StateHandlers{
            "invoices exist for tenant": func(setup bool, s pact.ProviderState) (pact.ProviderStateResponse, error) {
                if setup {
                    seedTestInvoices(testTenantID)
                } else {
                    cleanupTestData(testTenantID)
                }
                return pact.ProviderStateResponse{}, nil
            },
        },
    })
    assert.NoError(t, err)
}
```

Consumer contracts are published to the Pact Broker after each successful consumer test run. Provider verification runs on every server PR. A server change that breaks a consumer contract fails the PR build.

### 42.3.3 Mobile Consumer Contracts

iOS and Android apps define their own Pact consumer files covering the fields they consume from each endpoint. Mobile contracts are generated from the SDK types that wrap the API responses.

---

## 42.4 SDUI Schema Contracts

### 42.4.1 Surface Definition JSON Schema

The SDUI component schema is maintained as a JSON Schema document:

```json
// sdui-schema.json (excerpt)
{
  "$schema": "https://json-schema.org/draft/2020-12",
  "$id": "https://schema.awoerp.dev/sdui/v1/surface",
  "type": "object",
  "required": ["surface_id", "version", "locale", "components"],
  "properties": {
    "surface_id": { "type": "string" },
    "version": { "type": "integer" },
    "locale": { "type": "string", "pattern": "^[a-z]{2}-[A-Z]{2}$" },
    "components": {
      "type": "array",
      "items": { "$ref": "#/$defs/ComponentNode" }
    }
  },
  "$defs": {
    "ComponentNode": {
      "type": "object",
      "required": ["type", "id"],
      "properties": {
        "type": { "type": "string" },
        "id": { "type": "string" },
        "children": { "type": "array", "items": { "$ref": "#/$defs/ComponentNode" } }
      },
      "discriminator": {
        "propertyName": "type",
        "mapping": {
          "layout.section": "#/$defs/LayoutSection",
          "action.button": "#/$defs/ActionButton",
          "data.table": "#/$defs/DataTable"
        }
      }
    }
  }
}
```

### 42.4.2 Schema Validation in Compilation

Every compiled surface definition is validated against the JSON Schema before being cached or returned:

```go
func (c *SurfaceCompiler) validate(def *SurfaceDefinition) error {
    data, _ := json.Marshal(def)
    result, err := c.schema.Validate(bytes.NewReader(data))
    if err != nil {
        return fmt.Errorf("schema validation error: %w", err)
    }
    if !result.Valid() {
        var errs []string
        for _, desc := range result.Errors() {
            errs = append(errs, desc.String())
        }
        return fmt.Errorf("compiled surface is invalid: %s", strings.Join(errs, "; "))
    }
    return nil
}
```

### 42.4.3 Client Schema Validation

Client apps validate incoming surface definitions against the schema on receipt. An invalid surface definition (schema violation) results in a surface load error, not a silent rendering failure. This prevents corrupted surfaces from reaching users without error.

---

## 42.5 Breaking Change Detection

### 42.5.1 oasdiff for OpenAPI

`oasdiff` detects breaking changes between the current OpenAPI spec and the base branch:

```bash
# In CI: compare current spec against main branch spec
oasdiff breaking \
    --base git://main:gen/http/openapi3.json \
    --revision gen/http/openapi3.json \
    --fail-on ERR  # Fail CI on any breaking change
```

Breaking changes detected:
- Removing an endpoint
- Removing a required request field
- Removing a response field
- Changing a response field's type
- Adding a new required request field
- Narrowing an enum (removing a valid value)

### 42.5.2 Approved Breaking Changes

When a breaking change is required (per the API versioning policy in Chapter 25), the following process applies:

1. New endpoint or field is added alongside the old one
2. Old endpoint/field gets `X-Deprecated` and `Sunset` headers (Chapter 25)
3. 180-day deprecation window begins
4. Consumers are notified and given time to migrate
5. Old endpoint/field is removed after the deprecation window

Breaking changes without this process block the PR.

---

## 42.6 Backwards Compatibility Tests

### 42.6.1 Client Version Compatibility Matrix

The API must support clients up to the current version minus 3 minor versions. A compatibility matrix test verifies that:

```go
func TestBackwardsCompatibility(t *testing.T) {
    clientVersions := []string{
        currentVersion,
        previousMinorVersion,
        twoMinorVersionsAgo,
        threeMinorVersionsAgo,
    }

    for _, version := range clientVersions {
        t.Run(fmt.Sprintf("client_version_%s", version), func(t *testing.T) {
            // Send request with X-Client-Version header for this version
            // Verify response is compatible with that version's expected shape
            resp := apiClient.GetSurface("invoice_list",
                withClientVersion(version),
            )
            assert.Equal(t, 200, resp.Status)
            assertResponseShapeFor(t, resp, version)
        })
    }
}
```

### 42.6.2 SDUI Schema Versioning

The SDUI schema version is embedded in each surface definition:

```json
{ "schema_version": "1.3", "surface_id": "invoice_list", ... }
```

Clients declare the schema versions they support. The compiler never emits a schema version newer than what the client's `X-Schema-Version` header advertises. If the client is too old to support a required schema feature, it receives an `upgrade_required` error.

---

*End of Chapter 42*

**Previous:** [Chapter 41 — Testing Strategy](./41-testing-strategy.md)
**Next:** [Chapter 43 — IDE Tooling](./43-ide-tooling.md)
