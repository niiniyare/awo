> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Getting Started"
id: mdg-01
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Define an Entity](02-define-entity.md)"
  - "[Module System](../10-modules/module-system.md)"
  - "[Startup Sequence](../03-kernel/startup-sequence.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Getting Started

**MDG-01 | Module Developer Guide**

This document walks through creating the scaffold for a new business module: directory structure, module manifest, and the `init()` registration entry point.

---

## 1. Create the Directory Structure

```bash
mkdir -p internal/core/crm/migrations
mkdir -p internal/core/crm/workflows
```

The complete layout you will build:

```
internal/core/crm/
    crm.go             ← init() — registers all definitions
    manifest.go        ← ModuleManifest
    def.go      ← EntityDefinition variables
    policy.go          ← PolicyFunc implementations
    hooks.go           ← Hook implementations
    service.go         ← Optional: thin service layer
    handler.go         ← Optional: custom HTTP handlers
    workflows/
        workflows.go   ← Workflow function declarations
        activities.go  ← Activities struct + activity methods
        register.go    ← RegisterActivities()
    migrations/
        (SQL files added in MDG-10)
    crm_test.go        ← Unit tests
```

---

## 2. Declare the Module Manifest

```go
// internal/core/crm/manifest.go
package crm

import "awo.so/awo/def"

var Manifest = def.ModuleManifest{
    Name:    "crm",
    Label:   "CRM",
    Version: "1.0.0",

    // Capability tokens this module provides
    Provides: []string{
        "crm.contacts",
        "crm.interactions",
    },

    // Capability tokens this module requires
    // platform.tenancy and platform.iam are available on every deployment
    Requires: []string{
        "platform.tenancy",
        "platform.iam",
    },

    Owner: "crm-team",
}
```

---

## 3. Create the init() Entry Point

```go
// internal/core/crm/crm.go
package crm

import "awo.so/awo/def"

func init() {
    // Register the module manifest
    def.RegisterManifest(&Manifest)

    // Register entity definitions (declarations added in subsequent steps)
    def.Register(&ContactDefinition)
    def.Register(&InteractionDefinition)
}
```

These registrations are stubs — `ContactDefinition` and `InteractionDefinition` will be declared in `def.go` in the next step.

---

## 4. Wire the Module into the Server

Add a blank import to `cmd/server/main.go`:

```go
import (
    // ... existing imports ...
    _ "awo.so/internal/core/crm"  // triggers crm.init()
)
```

And register activities in the Temporal worker setup:

```go
// cmd/server/main.go — worker setup section
crm.RegisterActivities(w, crmDependencies)
```

---

## 5. Verify the Scaffold

At this point the module scaffold exists. The server will not compile yet (ContactDefinition is not declared). Proceed to [Define an Entity](02-define-entity.md).

**Dependency chain established:**
- `def.Register()` queues definitions for `Registry.Compile()`
- `Registry.Compile()` runs during startup Step 4
- Until `ContactDefinition` is declared, the build will fail with an undefined symbol

---

## Next: [Define an Entity →](02-define-entity.md)
