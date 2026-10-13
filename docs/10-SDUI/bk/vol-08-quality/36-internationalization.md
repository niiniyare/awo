# Chapter 36 — Internationalization

> **Volume:** VIII — Quality, Observability & Operations
> **Phase:** 4
> **Audience:** Frontend Engineers, Backend Engineers, Platform Engineers
> **Prerequisites:** Chapters 08–12, 17–21

---

## Table of Contents

- [36.1 i18n Philosophy](#361-i18n-philosophy)
- [36.2 Locale Resolution](#362-locale-resolution)
- [36.3 Translation Strings](#363-translation-strings)
- [36.4 Date, Time, and Timezone Formatting](#364-date-time-and-timezone-formatting)
- [36.5 Number and Currency Formatting](#365-number-and-currency-formatting)
- [36.6 Right-to-Left (RTL) Support](#366-right-to-left-rtl-support)
- [36.7 SDUI Localization](#367-sdui-localization)
- [36.8 Backend Localization](#368-backend-localization)
- [36.9 Locale Testing](#369-locale-testing)

---

## 36.1 i18n Philosophy

AwoERP is built for East African and broader African markets. English is the default, but the platform is designed so that every user-facing string, number, date, and currency can be presented in the user's locale without code changes — only translation files and locale configuration.

**Guiding principles:**

1. **No string literals in rendering code.** Every user-facing string is a translation key. A string not backed by a key is a bug.
2. **Server controls locale, client renders.** The compiled surface definition carries the locale for the session. Clients apply locale-aware formatting to the values they receive.
3. **Tenant can override platform labels.** Chapter 30 (Customization) describes label overrides. i18n and label overrides compose — a tenant-overridden label is still localizable.
4. **Monetary amounts carry explicit currency codes.** The platform never assumes a currency. Every amount in the API includes a `currency_code` (ISO 4217). Clients format it with the user's locale rules for that currency.

**Supported locales at launch:**

| Locale | Language | Market |
|--------|----------|--------|
| `en-KE` | English (Kenya) | Kenya |
| `en-NG` | English (Nigeria) | Nigeria |
| `en-GH` | English (Ghana) | Ghana |
| `en-TZ` | English (Tanzania) | Tanzania |
| `sw-KE` | Swahili (Kenya) | Kenya |
| `sw-TZ` | Swahili (Tanzania) | Tanzania |
| `fr-SN` | French (Senegal) | Senegal, francophone West Africa |
| `ar-EG` | Arabic (Egypt) | MENA expansion |

---

## 36.2 Locale Resolution

### 36.2.1 Resolution Order

The user's locale is resolved server-side at session time using the following priority order:

1. **User profile preference** — explicitly set by the user in their account settings
2. **Tenant default locale** — set by the tenant admin; applies when the user has no preference
3. **Browser / OS locale** — extracted from `Accept-Language` header (web) or device locale (mobile)
4. **Platform default** — `en-KE`

```go
func resolveLocale(userPreference, tenantDefault, acceptLanguage string) string {
    if userPreference != "" && isSupportedLocale(userPreference) {
        return userPreference
    }
    if tenantDefault != "" && isSupportedLocale(tenantDefault) {
        return tenantDefault
    }
    // Parse Accept-Language header, try each in quality order
    for _, lang := range parseAcceptLanguage(acceptLanguage) {
        if isSupportedLocale(lang) {
            return lang
        }
        // Try language-only match (e.g., "sw" → "sw-KE")
        if fallback, ok := languageFallback[lang]; ok {
            return fallback
        }
    }
    return "en-KE"
}
```

### 36.2.2 Locale in Session Context

The resolved locale is stored in the session context and included in the surface definition compilation key:

```go
type SessionContext struct {
    UserID    string
    TenantID  string
    Roles     []string
    Locale    string   // e.g., "sw-KE"
    Timezone  string   // e.g., "Africa/Nairobi"
    // ...
}
```

The `locale` is part of the SDUI cache key (Chapter 34.2.1), so a Swahili user and an English user on the same tenant receive differently compiled surface definitions.

---

## 36.3 Translation Strings

### 36.3.1 Translation File Structure

Translation files are JSON, organized by locale and namespace:

```
translations/
├── en-KE/
│   ├── common.json       ← Shared labels (Save, Cancel, Delete, Loading…)
│   ├── invoices.json     ← Invoice-specific strings
│   ├── vendors.json
│   └── errors.json       ← Error messages
├── sw-KE/
│   ├── common.json
│   ├── invoices.json
│   └── ...
└── fr-SN/
    └── ...
```

### 36.3.2 Translation Key Format

Keys are dot-separated, namespaced strings:

```json
// en-KE/invoices.json
{
  "invoice.create.title": "Create Invoice",
  "invoice.create.vendor_label": "Vendor",
  "invoice.create.amount_label": "Amount",
  "invoice.status.draft": "Draft",
  "invoice.status.pending_approval": "Pending Approval",
  "invoice.status.approved": "Approved",
  "invoice.action.approve": "Approve Invoice",
  "invoice.action.approve_confirm": "Are you sure you want to approve invoice {{invoice_number}} for {{amount}}?",
  "invoice.error.amount_required": "Amount is required",
  "invoice.error.amount_positive": "Amount must be greater than zero"
}
```

```json
// sw-KE/invoices.json
{
  "invoice.create.title": "Unda Ankara",
  "invoice.create.vendor_label": "Muuzaji",
  "invoice.create.amount_label": "Kiasi",
  "invoice.status.draft": "Rasimu",
  "invoice.status.pending_approval": "Inasubiri Idhini",
  "invoice.status.approved": "Imeidhinishwa",
  "invoice.action.approve": "Idhinisha Ankara",
  "invoice.action.approve_confirm": "Una uhakika unataka kuidhinisha ankara {{invoice_number}} ya {{amount}}?"
}
```

### 36.3.3 Interpolation

Translation strings support `{{variable}}` interpolation:

```typescript
// Web (react-i18next)
import { useTranslation } from 'react-i18next';

function ApproveConfirmDialog({ invoice }: { invoice: Invoice }) {
    const { t } = useTranslation('invoices');
    return (
        <Dialog>
            <p>{t('invoice.action.approve_confirm', {
                invoice_number: invoice.number,
                amount: formatCurrency(invoice.amount, invoice.currency_code, locale),
            })}</p>
        </Dialog>
    );
}
```

### 36.3.4 Pluralization

Pluralization rules vary by locale. The platform uses the ICU message format for plural strings:

```json
{
  "invoice.count": "{count, plural, =0 {No invoices} one {# invoice} other {# invoices}}",
  "invoice.count.sw": "{count, plural, other {Ankara # }}"
}
```

---

## 36.4 Date, Time, and Timezone Formatting

### 36.4.1 Timezone Handling

All timestamps are stored and transmitted as UTC. Formatting for display is done on the client using the session's resolved timezone.

```typescript
// All API responses return UTC ISO 8601
// "created_at": "2024-03-15T11:32:00Z"

function formatDateTime(utcString: string, locale: string, timezone: string): string {
    return new Intl.DateTimeFormat(locale, {
        dateStyle: 'medium',
        timeStyle: 'short',
        timeZone: timezone,
    }).format(new Date(utcString));
}

// In Nairobi (Africa/Nairobi, UTC+3):
// formatDateTime("2024-03-15T11:32:00Z", "sw-KE", "Africa/Nairobi")
// → "15 Mac 2024, 14:32"
```

### 36.4.2 Relative Time

Relative time ("3 hours ago", "yesterday") uses `Intl.RelativeTimeFormat`:

```typescript
function formatRelativeTime(utcString: string, locale: string): string {
    const diff = (new Date(utcString).getTime() - Date.now()) / 1000;
    const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });

    if (Math.abs(diff) < 60) return rtf.format(Math.round(diff), 'second');
    if (Math.abs(diff) < 3600) return rtf.format(Math.round(diff / 60), 'minute');
    if (Math.abs(diff) < 86400) return rtf.format(Math.round(diff / 3600), 'hour');
    return rtf.format(Math.round(diff / 86400), 'day');
}
```

### 36.4.3 Fiscal Calendar

Some tenants operate on a non-calendar fiscal year. The tenant configuration includes:

```json
{
  "fiscal_year": {
    "start_month": 7,
    "start_day": 1
  }
}
```

Date labels in reports ("FY2024 Q1") use the tenant's fiscal calendar, not the Gregorian calendar. The surface compiler injects the fiscal calendar configuration into the session context.

---

## 36.5 Number and Currency Formatting

### 36.5.1 Number Formatting

Number formatting (decimal separator, thousands separator) follows the user's locale:

```typescript
function formatNumber(value: number, locale: string, options?: Intl.NumberFormatOptions): string {
    return new Intl.NumberFormat(locale, options).format(value);
}

// en-KE: "1,234,567.89"
// fr-SN: "1 234 567,89"
```

### 36.5.2 Currency Formatting

All monetary amounts in the API carry an explicit `currency_code`. The client formats the amount according to the locale's rules for that currency:

```typescript
function formatCurrency(amount: number, currencyCode: string, locale: string): string {
    return new Intl.NumberFormat(locale, {
        style: 'currency',
        currency: currencyCode,
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
    }).format(amount);
}

// formatCurrency(150000, 'KES', 'en-KE') → "KSh 150,000.00"
// formatCurrency(150000, 'NGN', 'en-NG') → "₦150,000.00"
// formatCurrency(150000, 'GHS', 'en-GH') → "GH₵150,000.00"
```

### 36.5.3 Currency in SDUI Components

SDUI components that display monetary values use the `currency` display type, which the rendering engine formats using the session's locale and the field's `currency_code` binding:

```json
{
  "type": "field.display",
  "label_key": "invoice.total_amount",
  "value_binding": "$row.total_amount",
  "display_type": "currency",
  "currency_code_binding": "$row.currency_code"
}
```

The rendering engine calls `formatCurrency(value, currencyCode, sessionLocale)`.

---

## 36.6 Right-to-Left (RTL) Support

Arabic (`ar-EG`) requires RTL text direction. The platform handles RTL at two levels:

### 36.6.1 HTML Direction

The `<html>` element's `dir` attribute is set based on the resolved locale:

```typescript
const RTL_LOCALES = new Set(['ar-EG', 'ar-SA', 'he-IL']);

function applyDirection(locale: string) {
    document.documentElement.dir = RTL_LOCALES.has(locale) ? 'rtl' : 'ltr';
    document.documentElement.lang = locale;
}
```

### 36.6.2 Layout Mirroring

CSS uses logical properties throughout (no `left`/`right`, only `start`/`end`):

```css
/* Not this */
.sidebar { margin-left: 16px; }

/* This */
.sidebar { margin-inline-start: 16px; }
```

The design system's spacing tokens all use logical properties. SDUI-generated layouts inherit the document direction automatically.

### 36.6.3 Mobile RTL

**iOS:** Set `semanticContentAttribute = .forceRightToLeft` on the root view for RTL locales.

**Android:** Set `android:layoutDirection="locale"` at the application level; Android handles RTL mirroring automatically when `supportsRtl="true"` is declared in `AndroidManifest.xml`.

---

## 36.7 SDUI Localization

### 36.7.1 String Keys in Surface Definitions

Surface definitions use translation keys rather than literal strings:

```json
{
  "type": "button",
  "label_key": "invoice.action.approve",
  "on_click_action_id": "action_approve_invoice"
}
```

The `label_key` is looked up in the translation file for the session's locale at render time. If a key is missing in the target locale, the `en-KE` fallback is used. Missing keys are logged as warnings in the telemetry pipeline.

### 36.7.2 Compiled Translation Bundle

The surface compiler includes the resolved locale's translation keys in the compiled surface definition. The client does not fetch separate translation files — they arrive pre-embedded in the surface definition:

```json
{
  "surface_id": "invoice_list",
  "locale": "sw-KE",
  "translations": {
    "invoice.action.approve": "Idhinisha Ankara",
    "invoice.status.draft": "Rasimu",
    "common.save": "Hifadhi",
    "common.cancel": "Ghairi"
  },
  "components": [ ... ]
}
```

Only the keys used in the surface are included (tree-shaken from the full translation bundle). This keeps the payload small.

### 36.7.3 Tenant Label Overrides and i18n

Tenant label overrides (Chapter 30) compose with i18n. The override resolution order at compile time:

1. Tenant override for this locale + key → use tenant override
2. Tenant override for `en-KE` + key → use as fallback for this locale if no locale-specific override
3. Platform translation for this locale + key → use platform string
4. Platform translation for `en-KE` + key → ultimate fallback

---

## 36.8 Backend Localization

### 36.8.1 Error Messages

API error messages are returned with a `message_key` alongside the human-readable `message`. Clients may display their own localized version of the error using the key:

```json
{
  "error": "validation_error",
  "message": "Amount must be greater than zero",
  "message_key": "invoice.error.amount_positive",
  "field_errors": {
    "amount": {
      "message": "Amount must be greater than zero",
      "message_key": "invoice.error.amount_positive"
    }
  }
}
```

### 36.8.2 Workflow Notifications

Workflow approval notifications (email, in-app) are sent in the recipient's locale. The Temporal workflow activity that sends notifications accepts a `locale` parameter:

```go
type SendApprovalNotificationInput struct {
    RecipientUserID string
    InvoiceID       string
    Locale          string  // recipient's resolved locale
    Timezone        string  // recipient's timezone
}
```

Email templates use Handlebars with locale-specific template files (one per supported locale).

### 36.8.3 Report Generation

Reports generated as PDFs (export functionality) are rendered in the requesting user's locale. Number, date, and currency formatting in the PDF uses the user's locale rules.

---

## 36.9 Locale Testing

### 36.9.1 Pseudo-Locale

During development, the `x-pseudo` locale is available. It wraps all translated strings with brackets and adds accented characters: `"Save"` → `"[Šàvé]"`. This makes it immediately obvious which strings are being translated and which are hard-coded.

### 36.9.2 Missing Key Detection

A CI check scans all surface definitions and component templates, extracts every `label_key` reference, and verifies that the key exists in all supported locales:

```bash
# In CI
go run ./tools/i18n-check \
  --surfaces ./surfaces/ \
  --translations ./translations/ \
  --locales en-KE,sw-KE,en-NG,fr-SN,ar-EG
```

The check fails if any key is missing from any locale. Missing keys block deployment.

### 36.9.3 RTL Layout Testing

The `ar-EG` locale is included in the visual regression test suite (Chapter 41). Screenshots in RTL layout are compared against baseline to detect regressions in mirroring.

---

*End of Chapter 36*

**Previous:** [Chapter 35 — Data Sync](../vol-07-performance/35-data-sync.md)
**Next:** [Chapter 37 — Accessibility](./37-accessibility.md)
