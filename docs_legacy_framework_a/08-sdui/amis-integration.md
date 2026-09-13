> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "amis Integration"
id: sdui-003
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Page Schema](page-schema.md)"
  - "[Page Builders](page-builders.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# amis Integration

**SDUI-003 | Status: Accepted | Stability: Stable**

This document specifies how the pinned amis SDK is configured, theming rules, dark mode implementation, and known limitations.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. SDK Pinning

The amis SDK is pinned in `web/sdk/`:
```
web/sdk/sdk.js     — amis core renderer
web/sdk/sdk.css    — amis styles
web/sdk/charts.js  — ECharts integration
```

The SDK MUST NOT be updated without a full compatibility audit of all PageBuilder output. The audit must verify that every page schema produced by every PageBuilderFunc renders correctly with the new SDK version.

Automated compatibility testing: before any SDK update, run the page schema test suite which renders every registered entity's default schemas and checks for amis error boundaries.

---

## 2. SDK Bootstrap

`web/pages/index.html` bootstraps amis:

```html
<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="/sdk/sdk.css" />
</head>
<body>
  <div id="root"></div>
  <script src="/sdk/sdk.js"></script>
  <script src="/sdk/charts.js"></script>
  <script>
    amis.embed('#root', {
      type: 'app',
      api: '/api/v1/schemas/app',
      theme: 'cxd',
    });
  </script>
</body>
</html>
```

`theme: 'cxd'` is the default amis theme. Other bundled themes: `antd`, `dark` (see §4 for dark mode caveats).

---

## 3. API Integration

amis fetches schemas from the schema endpoints and data from the entity API endpoints. The integration is configured via amis `api` objects in the page schema:

```json
{
  "type": "crud",
  "api": {
    "method": "GET",
    "url": "/api/v1/entities/finance_invoice",
    "headers": {
      "X-Tenant-ID": "${tenantId}",
      "Authorization": "Bearer ${sessionToken}"
    }
  }
}
```

Session token and tenant ID are stored in the amis `data` context and injected into all API requests via the amis `api` object's `headers` configuration.

---

## 4. Theming

### Light Mode

The default theme (`cxd`) provides a clean light UI. CSS custom properties for the `cxd` theme are defined in `sdk.css`. Override at the `html` root:

```css
html {
  --colors-brand-5: #1890ff;  /* primary color */
  --colors-brand-6: #096dd9;  /* primary hover */
}
```

### Dark Mode

**There is no built-in dark CSS in amis for the `cxd` theme.** Using `theme: 'dark'` applies `classPrefix: "dark-"` to all component class names but provides zero CSS rules for those prefixed classes — components render with broken or missing styles.

The correct dark mode approach: override CSS custom property tokens at `html.dark`:

```css
/* Applied when <html class="dark"> is present */
html.dark {
  /* Neutral scale — invert: 1=darkest, 11=lightest in light mode */
  --colors-neutral-fill-1:  #262626;
  --colors-neutral-fill-2:  #303030;
  --colors-neutral-fill-11: #fafafa;
  --colors-neutral-text-2:  #d9d9d9;
  --colors-neutral-line-8:  #424242;

  /* Component backgrounds */
  --background:        #1a1a1a;
  --body-bg:           #141414;
  --Page-main-bg:      #1f1f1f;
  --Panel-bg-color:    #262626;
  --Table-bg:          #1f1f1f;
}
```

Toggle dark mode by adding/removing the `dark` class on `<html>`:

```javascript
document.documentElement.classList.toggle('dark');
```

Do NOT override individual `.cxd-*` component backgrounds with `!important` — this approach breaks on every SDK update and misses dynamically rendered components.

---

## 5. Known Limitations

| Limitation | Workaround |
|---|---|
| No real-time data updates | Use amis `interval` refresh on specific components |
| No WebSocket support | Custom JS outside amis bootstrap (not part of SDUI) |
| No drag-and-drop reordering | Not available; use sortable list via API ordering |
| Mobile layout limited | amis is desktop-first; basic mobile support only |
| Complex conditional form logic | Use amis `visibleOn` / `disabledOn` expressions |
| Custom chart types | ECharts `custom` series via amis chart component |
| File upload size limits | Configure Fiber's body limit and amis upload API |

---

## 6. amis Expression Language

amis uses a template expression language for dynamic values in schemas: `${expression}`. This executes in the browser (not on the server). Examples:

```json
{"visibleOn": "${status === 'Draft'}"}
{"disabledOn": "${status !== 'Draft'}"}
{"tpl": "Total: ${total_kes | number: 2}"}
```

PageBuilderFunc code in Go MUST NOT generate JavaScript expressions that depend on server-side data (use server-side rendering for that). amis expressions are purely for client-side conditional display.

---

## Related Documents

- [Page Schema](page-schema.md) — how schemas are generated and served
- [Page Builders](page-builders.md) — building custom schemas in Go
- [Glossary](../GLOSSARY.md) — amis, amis SDK, SDUI, Page Schema
