> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Configuration — Section Overview"
id: cfg-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Configuration](configuration.md)"
  - "[Startup Sequence](../03-kernel/startup-sequence.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Configuration

**Section 12 | Configuration**

Configuration covers process-level settings loaded at startup from environment variables. Application-level settings (tenant-overridable, runtime-changeable) are managed by the Settings platform module.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Configuration](configuration.md) | CFG-001 | Typed config struct, env vars reference, validation rules, secrets management | STABLE |
| [Feature Flags Configuration](feature-flags-config.md) | CFG-002 | Flag declaration, evaluation order, caching, lifecycle | STABLE |
| [Environment Variables Reference](environment-variables.md) | CFG-003 | Complete env var reference: required, optional, integration, Vault paths | STABLE |

---

## Prerequisites

- [Startup Sequence](../03-kernel/startup-sequence.md) — when config is loaded and validated
- [Glossary](../GLOSSARY.md) — Configuration, Config Struct
