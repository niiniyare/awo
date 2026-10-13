# Chapter 37 — Accessibility

> **Volume:** VIII — Quality, Observability & Operations
> **Phase:** 4
> **Audience:** Frontend Engineers, Platform Engineers, QA Engineers
> **Prerequisites:** Chapters 09–12, 17–21

---

## Table of Contents

- [37.1 Accessibility Standard](#371-accessibility-standard)
- [37.2 Semantic HTML and ARIA](#372-semantic-html-and-aria)
- [37.3 Keyboard Navigation](#373-keyboard-navigation)
- [37.4 Screen Reader Support](#374-screen-reader-support)
- [37.5 Color and Contrast](#375-color-and-contrast)
- [37.6 Mobile Accessibility](#376-mobile-accessibility)
- [37.7 SDUI Accessibility Model](#377-sdui-accessibility-model)
- [37.8 Accessibility Testing](#378-accessibility-testing)

---

## 37.1 Accessibility Standard

AwoERP targets **WCAG 2.1 Level AA** conformance for all web surfaces. Mobile clients follow the platform's native accessibility guidelines (iOS Human Interface Guidelines, Android Accessibility Guide).

Level AA is the minimum. The platform aims for AAA where feasible without significant design compromise, particularly for color contrast (3:1 ratio required for AA, 4.5:1 preferred) and text sizing.

The SDUI model provides an accessibility advantage: accessibility attributes are authored once in the surface definition and rendered correctly by every client platform. The alternative — each client platform independently implementing accessible UI from scratch for every surface — would produce inconsistent results.

---

## 37.2 Semantic HTML and ARIA

### 37.2.1 Component Semantic Mapping

Each SDUI component type maps to a specific HTML element or ARIA role. The rendering engine uses this mapping; component authors never emit raw `<div>` elements where a semantic element exists:

| SDUI Component | HTML Element / ARIA Role |
|----------------|--------------------------|
| `layout.page` | `<main>` |
| `layout.section` | `<section>` with `aria-labelledby` |
| `layout.nav` | `<nav>` |
| `data.table` | `<table>` with `<caption>` |
| `data.table_row` | `<tr>` |
| `data.table_cell` | `<td>` / `<th scope="col">` |
| `input.text` | `<input type="text">` with associated `<label>` |
| `input.select` | `<select>` with associated `<label>` |
| `input.checkbox` | `<input type="checkbox">` with associated `<label>` |
| `action.button` | `<button type="button">` |
| `action.submit` | `<button type="submit">` |
| `action.link` | `<a>` with `href` |
| `feedback.alert` | `role="alert"` with `aria-live="assertive"` |
| `feedback.status` | `role="status"` with `aria-live="polite"` |
| `feedback.dialog` | `<dialog>` with `aria-modal="true"` |
| `navigation.tabs` | `role="tablist"` + `role="tab"` + `role="tabpanel"` |
| `navigation.breadcrumb` | `<nav aria-label="Breadcrumb">` + `<ol>` |

### 37.2.2 Labels

Every interactive element has an accessible label. The SDUI schema requires either `label_key` (translated text label) or `aria_label_key` (for icon-only buttons):

```json
{
  "type": "action.button",
  "label_key": "invoice.action.approve",
  "on_click_action_id": "action_approve_invoice"
}
```

```json
{
  "type": "action.button",
  "icon": "trash",
  "aria_label_key": "common.delete_row",
  "on_click_action_id": "action_delete_row"
}
```

The surface compiler validates that `label_key` or `aria_label_key` is present on all interactive components. Missing labels are a compile-time error, not a runtime warning.

### 37.2.3 Form Associations

All form inputs are associated with their labels programmatically:

```typescript
// Rendering engine generates:
// <label htmlFor="field-invoice-amount">Amount</label>
// <input id="field-invoice-amount" type="text" aria-describedby="field-invoice-amount-hint field-invoice-amount-error" />
// <span id="field-invoice-amount-hint">Enter amount in KES</span>
// <span id="field-invoice-amount-error" role="alert">Amount must be greater than zero</span>

function renderFormField(field: FormFieldNode, locale: string): React.ReactElement {
    const fieldId = `field-${field.id}`;
    const hintId = field.hint_key ? `${fieldId}-hint` : undefined;
    const errorId = field.error ? `${fieldId}-error` : undefined;

    const describedBy = [hintId, errorId].filter(Boolean).join(' ');

    return (
        <div>
            <label htmlFor={fieldId}>{t(field.label_key)}</label>
            <input
                id={fieldId}
                type={field.input_type}
                aria-describedby={describedBy || undefined}
                aria-invalid={!!field.error}
                aria-required={field.required}
            />
            {hintId && <span id={hintId}>{t(field.hint_key!)}</span>}
            {errorId && <span id={errorId} role="alert">{field.error}</span>}
        </div>
    );
}
```

---

## 37.3 Keyboard Navigation

### 37.3.1 Focus Management

All interactive elements are keyboard reachable. Tab order follows the DOM order, which the SDUI rendering engine ensures matches the visual order.

**Focus trapping in dialogs:** When a dialog opens, focus is moved to the first focusable element within it. Tab cycles within the dialog. Escape closes the dialog and returns focus to the triggering element.

```typescript
function Dialog({ open, onClose, children, triggerRef }: DialogProps) {
    const dialogRef = useRef<HTMLDialogElement>(null);

    useEffect(() => {
        if (open) {
            dialogRef.current?.showModal();
            // Focus first focusable element
            const first = dialogRef.current?.querySelector<HTMLElement>(FOCUSABLE_SELECTORS);
            first?.focus();
        } else {
            dialogRef.current?.close();
            triggerRef.current?.focus();  // Return focus to trigger
        }
    }, [open]);

    return (
        <dialog ref={dialogRef} onKeyDown={e => e.key === 'Escape' && onClose()}>
            {children}
        </dialog>
    );
}
```

### 37.3.2 Keyboard Shortcuts

Data tables support keyboard navigation:
- `↑` / `↓` — move between rows
- `Enter` / `Space` — activate the row's primary action
- `Home` / `End` — jump to first / last row
- `/` — focus the table's search input (when present)

Keyboard shortcuts are discoverable via a `?` shortcut panel (accessible from the keyboard and announced by screen readers).

### 37.3.3 Skip Links

Every page includes a skip link as the first focusable element, allowing keyboard users to bypass navigation:

```html
<a href="#main-content" class="skip-link">Skip to main content</a>
```

The skip link is visually hidden until focused.

---

## 37.4 Screen Reader Support

### 37.4.1 Live Regions

Dynamic content updates (SSE patches, loading states, success/error notifications) use ARIA live regions so screen readers announce changes without requiring focus:

```typescript
// Success toast
<div role="status" aria-live="polite" aria-atomic="true">
    {successMessage}
</div>

// Error alert
<div role="alert" aria-live="assertive" aria-atomic="true">
    {errorMessage}
</div>

// Loading state
<div aria-live="polite" aria-busy={isLoading}>
    {isLoading ? <span className="sr-only">Loading...</span> : content}
</div>
```

### 37.4.2 Loading States

When a surface or data source is loading, a `sr-only` (visually hidden) text is announced:

```json
{
  "type": "feedback.loading",
  "aria_label_key": "common.loading_data"
}
```

Renders as `<div aria-busy="true"><span class="sr-only">Loading data…</span><Spinner /></div>`.

### 37.4.3 Table Accessibility

Data tables include a `<caption>` for screen reader context and use `scope` on column headers:

```typescript
function DataTable({ definition, rows }: TableProps) {
    return (
        <table>
            <caption className="sr-only">{t(definition.caption_key)}</caption>
            <thead>
                <tr>
                    {definition.columns.map(col => (
                        <th key={col.id} scope="col" aria-sort={col.sortState}>
                            {t(col.label_key)}
                        </th>
                    ))}
                </tr>
            </thead>
            <tbody>
                {rows.map(row => (
                    <tr key={row.id} aria-selected={row.isSelected}>
                        {/* cells */}
                    </tr>
                ))}
            </tbody>
        </table>
    );
}
```

Sortable columns use `aria-sort="ascending"` / `"descending"` / `"none"`.

---

## 37.5 Color and Contrast

### 37.5.1 Contrast Ratios

| Element | WCAG AA Requirement | AwoERP Target |
|---------|-------------------|---------------|
| Normal text (<18px) | 4.5:1 | 7:1 |
| Large text (≥18px bold or ≥24px) | 3:1 | 4.5:1 |
| UI components (borders, icons) | 3:1 | 4.5:1 |
| Disabled elements | No requirement | Not tested |

The design system's color tokens are defined with their contrast ratios documented. A CI check runs `axe-core` contrast checks against all rendered surfaces.

### 37.5.2 Color is Not the Only Signal

No information is conveyed by color alone. Status indicators combine color with an icon and text label:

```json
{
  "type": "badge.status",
  "status_binding": "$row.status",
  "display_map": {
    "draft":            { "color": "gray",   "icon": "draft",   "label_key": "invoice.status.draft" },
    "pending_approval": { "color": "amber",  "icon": "clock",   "label_key": "invoice.status.pending_approval" },
    "approved":         { "color": "green",  "icon": "check",   "label_key": "invoice.status.approved" },
    "rejected":         { "color": "red",    "icon": "x",       "label_key": "invoice.status.rejected" }
  }
}
```

### 37.5.3 Focus Indicators

Focus rings are always visible, with a minimum 3:1 contrast ratio against the adjacent background. The default browser focus ring is replaced with a custom 2px offset ring using the brand's focus color:

```css
:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
}
```

`outline: none` is never used without a replacement focus style.

---

## 37.6 Mobile Accessibility

### 37.6.1 iOS VoiceOver

Every SDUI component that renders on iOS maps to an accessible iOS view:

```swift
// SDUI rendering engine maps component nodes to UIView with accessiblity properties
func renderComponent(_ node: ComponentNode) -> UIView {
    switch node.type {
    case "action.button":
        let button = UIButton()
        button.accessibilityLabel = t(node.ariaLabelKey ?? node.labelKey)
        button.accessibilityTraits = .button
        if node.isDisabled {
            button.accessibilityTraits.insert(.notEnabled)
        }
        return button

    case "feedback.alert":
        let label = UILabel()
        label.accessibilityLabel = label.text
        UIAccessibility.post(notification: .announcement, argument: label.text)
        return label

    default:
        return UIView()
    }
}
```

### 37.6.2 Android TalkBack

Android renders SDUI nodes with content descriptions and accessibility actions:

```kotlin
fun ComponentNode.toAccessibleView(context: Context): View {
    val view = when (type) {
        "action.button" -> Button(context).apply {
            contentDescription = t(ariaLabelKey ?: labelKey)
            isEnabled = !isDisabled
        }
        "data.table_row" -> LinearLayout(context).apply {
            importantForAccessibility = IMPORTANT_FOR_ACCESSIBILITY_YES
            contentDescription = buildRowDescription(this@toAccessibleView)
        }
        else -> View(context)
    }
    return view
}
```

### 37.6.3 Touch Target Size

All interactive elements have a minimum touch target of 44×44pt (iOS) / 48×48dp (Android), per platform guidelines. Smaller visual elements are padded to meet the minimum.

---

## 37.7 SDUI Accessibility Model

### 37.7.1 Accessibility Attributes in Surface Definitions

Surface definitions support explicit accessibility overrides for edge cases where the default semantic mapping is insufficient:

```json
{
  "type": "layout.section",
  "id": "approval_section",
  "accessibility": {
    "role": "region",
    "aria_label_key": "invoice.approval.section_label",
    "aria_live": "polite"
  },
  "children": [ ... ]
}
```

### 37.7.2 Compile-Time Accessibility Validation

The surface compiler runs accessibility checks as part of compilation:

```go
type AccessibilityValidator struct{}

func (v *AccessibilityValidator) Validate(node ComponentNode) []ValidationError {
    var errors []ValidationError

    // Interactive components must have a label
    if isInteractive(node.Type) {
        if node.LabelKey == "" && node.AriaLabelKey == "" {
            errors = append(errors, ValidationError{
                Code:    "a11y_missing_label",
                Message: fmt.Sprintf("component %s (%s) has no label_key or aria_label_key", node.ID, node.Type),
                Level:   ErrorLevelError,
            })
        }
    }

    // Images must have alt text or be marked decorative
    if node.Type == "media.image" {
        if node.AltTextKey == "" && !node.IsDecorative {
            errors = append(errors, ValidationError{
                Code:    "a11y_missing_alt",
                Message: fmt.Sprintf("image %s has no alt_text_key and is not marked decorative", node.ID),
                Level:   ErrorLevelError,
            })
        }
    }

    return errors
}
```

Accessibility validation errors block surface publication.

---

## 37.8 Accessibility Testing

### 37.8.1 Automated Testing

**Web:** `axe-core` runs in the CI pipeline against every rendered surface (via Playwright). It checks for WCAG 2.1 AA violations automatically. New violations block deployment.

```typescript
// In Playwright test
import { checkA11y } from 'axe-playwright';

test('invoice list is accessible', async ({ page }) => {
    await page.goto('/invoices');
    await checkA11y(page, null, {
        runOnly: {
            type: 'tag',
            values: ['wcag2a', 'wcag2aa'],
        },
    });
});
```

**Mobile:** iOS `XCUIAccessibilityAudit` and Android `AccessibilityChecks` run in the mobile test suite.

### 37.8.2 Manual Testing Protocol

Automated tools catch ~30-40% of WCAG failures. The QA checklist for each new surface includes:

- [ ] Keyboard-only navigation: complete all primary tasks without a mouse
- [ ] VoiceOver (iOS) and TalkBack (Android): navigate and complete a primary task
- [ ] NVDA + Chrome (Windows): navigate the surface and complete a form
- [ ] 200% browser zoom: no content clipping or overlap
- [ ] High contrast mode (Windows): all UI elements remain visible
- [ ] Verify all form errors are announced without requiring focus on the error

### 37.8.3 Accessibility in the Definition Review Process

Surface definitions are reviewed for accessibility as part of the platform review process before publication. Reviewers check that:
- All interactive components have label keys
- Images have alt text or are marked decorative
- Status information uses color + icon + text (not color alone)
- Dynamic regions use appropriate `aria-live` values

---

*End of Chapter 37*

**Previous:** [Chapter 36 — Internationalization](./36-internationalization.md)
**Next:** [Chapter 38 — Observability](./38-observability.md)
