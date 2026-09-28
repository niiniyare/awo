# Enterprise-grade ERP UI standard

## Purpose and scope

This is the yardstick for AWO's auto-generated CRUD pages: what an enterprise-grade ERP page must have, what is nice to have, and what leading systems are moving toward. Use it as a review checklist when changing the SDUI generator.

- **In scope:** pages generated from an EntityDefinition (list, create/edit form, detail, module workspace).
- **Out of scope:** bespoke pages built separately with a PageBuilder (POS screens, custom dashboards, wizards). They may borrow from this standard but are not held to it.
- **Tiers:** Tier 1 = necessary; users read its absence as a defect. Tier 2 = useful, adds polish or speed. Tier 3 = emerging; few systems do it well yet.
- **Evidence:** competitor claims cite vendor documentation. Anything not verified is marked *unverified* rather than asserted.

## Page archetypes

Every enterprise ERP reduces to four page types, each with the same regions in the same order, so users learn one layout once. Consistency across entities is the main thing generated pages must deliver.

| Archetype | Regions, top to bottom | Primary job |
| --- | --- | --- |
| List / worklist | Breadcrumb and title; view switcher and saved views; filter bar; action bar (create, bulk, export, columns); table with sortable columns, status badges and row actions; pagination and count | Find, triage and act on many records |
| Form (create / edit) | Breadcrumb and title; status and primary actions; sections or tabs of fields; validation summary; save / cancel bar that guards unsaved changes | Enter or change one record correctly |
| Detail (read) | Breadcrumb and title; header summary with status and key figures; workflow or next-step actions; section tabs; related lists; activity and audit timeline; attachments | Understand one record and decide what to do next |
| Workspace / dashboard | Title and period; KPI cards; charts with drill-through; short worklists (my approvals, overdue); shortcuts | Answer "what needs my attention today" |

## Feature tiers

Tier 1 is the bar for calling a page enterprise-grade; a missing item there reads as a bug, not a gap. Tiers 2 and 3 are chosen deliberately.

### Tier 1: necessary

| Feature | Why it is necessary |
| --- | --- |
| Server-side pagination, sorting, filtering | Tables hold hundreds of thousands of rows |
| Quick search plus structured filters | Finding a record is the most common task |
| Saved views / variants per user | Users repeat the same filter every day (SAP variants, D365 saved views) |
| Column chooser and resizing | Different roles need different columns |
| Bulk actions with selection kept across pages | Approving or deleting one by one does not scale |
| Export (CSV / Excel) | Finance and audit always leave the app for a spreadsheet |
| Permission-aware actions and fields | Users must not see buttons they cannot use |
| Inline field validation and server error mapping | Errors shown at the field, before and after submit |
| Unsaved-changes guard | Prevents silent data loss |
| Conditional required / read-only / hidden fields | Forms adapt to document type and status |
| Status badges and a visible lifecycle | Draft, submitted, approved, cancelled must be obvious |
| Breadcrumbs and consistent titles | Orientation in deep module trees |
| Empty, loading and error states | Avoids blank screens and dead ends |
| Audit trail / activity log per record | Compliance and dispute resolution |
| Attachments and notes on records | Invoices, contracts and comments live with the record |
| Related records and drill-through | Order to invoice to payment without searching |
| Multi-tenant / company / currency context | Every figure needs its entity and currency |
| Localized dates, numbers, currency, RTL | Multi-country deployments |
| Keyboard operation and WCAG 2.1 AA | Data-entry speed and legal accessibility |
| Responsive layout | Tablets on warehouse and forecourt floors |

### Tier 2: useful, not necessary

| Feature | Value |
| --- | --- |
| Kanban, calendar, pivot and graph views | Different mental models for the same data (Odoo, Frappe) |
| Group-by with subtotals | Quick analysis without a report |
| Inline grid editing | Fast mass corrections |
| Side-panel quick view / split view | Preview without leaving the list |
| Fact boxes for related summaries | Context beside the record (D365) |
| Pinned favorites and recent items | Fast return to daily work |
| Print / PDF templates | Legal documents and delivery notes |
| Duplicate record, import wizard, bulk edit | Setup and migration speed |
| Follow / subscribe and @mentions | Collaboration around records (Odoo chatter) |
| Per-user personalization, density and dark mode | Comfort for all-day use |
| Tags and colour labels | Lightweight categorisation |

### Tier 3: emerging

| Feature | Direction |
| --- | --- |
| Natural-language search and command palette | Ask or jump instead of navigating menus |
| AI copilot side panel | Summaries, drafting, guided actions (D365 Copilot) |
| AI-assisted data entry and document capture | Invoice and receipt extraction |
| Proactive insights and anomaly flags on lists | The page tells you what looks wrong |
| Real-time presence and collaboration | Who else is editing this record |
| Embedded analytics with drill-through | Dashboards inside the workflow |
| End-user low-code personalization | Users add fields and views without IT |
| Approval and workflow timeline visualization | See where a document is stuck and who holds it |
| Offline-capable PWA | Field and warehouse work without signal |
| Per-tenant branding and theming | White-label SaaS |

## Comparison with existing ERPs

Every major suite generates its standard pages from metadata, as AWO does, and they converge on saved views plus an AI assistant; open-source systems lead on alternate views and collaboration, closed suites on personalization and AI. Cells list only what the cited page states; anything else is *unverified*.

| System | Signature UX worth copying | Saved views / personalization | AI direction | Gap or weakness |
| --- | --- | --- | --- | --- |
| [Odoo 18](https://www.odoo.com/documentation/18.0/applications/essentials/search.html) (open source) | One search bar with Filter, Group By, Favorites; kanban, pivot, graph and activity views; chatter on records | Favorites store filters, grouping and column visibility; can be shared ([Odoo docs](https://www.odoo.com/documentation/18.0/developer/reference/user_interface/view_architectures.html)) | *Unverified* | *Unverified* |
| [ERPNext / Frappe](https://frappe.io/framework/desk-ui) (open source) | Desk auto-builds list, report, tree, kanban and calendar views from each DocType; the model AWO follows | v16 adds list-view and child-table enhancements ([Frappe v16 highlights](https://sanskartechnolab.com/frappe-framework/frappe-framework-v16)) | *Unverified* | *Unverified* |
| [Dolibarr](https://wiki.dolibarr.org/index.php?title=Module_Exports_En) (open source) | Saved export profiles (CSV, Excel); mass actions on list pages | *Unverified* | *Unverified* | *Unverified* |
| [metasfresh](https://docs.metasfresh.org/webui_collection/EN/Quickstart.html) (open source) | Any feature within 1-2 clicks; autocomplete, auto-load and auto-save on forms | *Unverified* | *Unverified* | *Unverified* |
| [Apache OFBiz](https://www.hotwaxsystems.com/hotwax-blog/apache-ofbiz-a-modern-framework-hidden-behind-a-generic-ui) (open source) | Solid backend; teams build Vue or React front ends over its REST APIs | Not in the default UI | *Unverified* | Generic default UI; polish is left to integrators |
| [SAP Fiori Elements](https://experience.sap.com/fiori-design-web/list-report-floorplan-sap-fiori-element/) (closed) | List report + object page floorplans generated from annotations; predefined views (All, Open, Assigned); chart and table views; anchor or tab sections | Variants and predefined views ([SAP](https://www.sap.com/design-system/fiori-design-web/v1-71/page-types/floorplans/list-report-floorplan-sap-fiori-element/usage)) | Joule: natural-language filters, page summaries, insight cards ([SAP Community](https://community.sap.com/t5/technology-blog-posts-by-sap/sap-ux-q1-2025-update-part-2-sap-s-4hana-cloud-public-edition-2502-and-sap/ba-p/14015395)) | *Unverified* |
| [Dynamics 365 Finance & Operations](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/dev-itpro/get-started/saved-views) (closed) | Saved views pinned to workspaces as tiles, lists or links; FactBoxes in a Related information pane | Saved views with filter, sort and default view; collapsible panes remembered per user ([Learn](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/fin-ops/get-started/user-interface-elements)) | Copilot chat in a right-hand pane ([Learn](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/dev-itpro/copilot/copilot-architecture)) | *Unverified* |
| [Dynamics 365 Business Central](https://learn.microsoft.com/en-gb/dynamics365/business-central/ui-personalization-user) (closed) | Personalization mode: move, hide, add fields and actions on any page; Edit in Excel from list pages | Per-user page personalization | Copilot Analysis Assist on lists: group, summarize, pivot (preview) ([Learn](https://learn.microsoft.com/en-us/dynamics365/business-central/analysis-assist)) | Analysis feature still in preview |
| [Oracle NetSuite](https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_N496945.html) (closed) | Saved searches double as list views and dashboard portlets; drag-and-drop dashboard personalization | Saved searches as preferred views | Ask Oracle assistant and new Redwood UI, rolling out through 2026 ([Houseblend](https://www.houseblend.io/articles/netsuite-next-ask-oracle-ai-interface)) | *Unverified* |
| [Workday](https://canvas.workday.com/) (closed) | Canvas design system, open source; consistent components | *Unverified* | Illuminate: anomaly detection, auto-fill, document scanning, Workday Assistant ([Workday](https://newsroom.workday.com/2024-09-17-Announcing-Workday-Illuminate-TM-The-Next-Generation-of-Workday-AI)) | *Unverified* |

Modern non-ERP tools (Linear, Airtable, Retool) were not researched here, so nothing is claimed about them.

## Where AWO is today

AWO fully covers 11 of 20 Tier 1 items and partly covers 5 more; the largest gaps are saved views, attachments and notes, and foreign-key labels. Status below was checked against `sdui/generator/generator.go` and `sdui/amis/renderer.go` on main at commit fe25734 (28 Sep 2026).

| Tier | Feature | Status | Notes | Fix lives in |
| --- | --- | --- | --- | --- |
| 1 | Pagination, sorting, filter bar | Done | Filter bar built from searchable fields; 20 rows per page, 10 to 100 selectable | sdui |
| 1 | Column chooser | Done | Column toggler on lists | sdui |
| 1 | Export | Partial | CSV of the current page, client-side; no server-side full export | sdui, then backend |
| 1 | Bulk actions | Partial | Bulk delete only; selection kept across pages | sdui |
| 1 | Permission-aware actions and fields | Done | Unauthorized nodes are absent, not hidden | sdui |
| 1 | Client-side validation | Done | Required, min, max, max length | sdui |
| 1 | Server error mapping | Partial | Field-error map exists in the API; not verified end to end in the UI | sdui and backend |
| 1 | Unsaved-changes guard | Done | Editable forms only | sdui |
| 1 | Conditional required, read-only, visible | Done | Expressions passed through to amis | sdui |
| 1 | Status badges | Done | Opt-in per field with a colour map | sdui and entity definitions |
| 1 | Breadcrumbs | Done | Home, list, current view | sdui |
| 1 | Empty, loading, error states | Partial | Empty state localizable; no loading or error customization | sdui |
| 1 | Audit trail per record | Done | Timeline on detail pages for audited entities | sdui |
| 1 | Attachments and notes | Missing | An attachments node exists in the renderer but the generator never emits it | sdui, plus attachment API |
| 1 | Related records | Done | Related lists from entity edges | sdui |
| 1 | Saved views / variants | Missing | No per-user or shared views | sdui, plus persistence API |
| 1 | Foreign-key labels in lists | Missing | Link columns show the raw id | backend list API, then sdui |
| 1 | Localization and RTL | Partial | Date, number and RTL formats for 30+ locales; strings are English only | sdui, plus translation content |
| 1 | Accessibility (WCAG 2.1 AA) | Unverified | Relies on amis defaults; never audited | sdui and web shell |
| 1 | Responsive layout | Done | 12, 8 and 4 column grid | sdui |
| 2 | Kanban, calendar, pivot, graph views | Missing | List, form, detail and dashboard only | sdui |
| 2 | Group-by, inline edit, side panel | Missing | | sdui |
| 2 | Print / PDF, import wizard, duplicate | Missing | | sdui plus backend |
| 2 | Dark mode and density | Partial | CSS overlay in the web shell; compact theme has no distinct styling | web shell |
| 3 | AI assistant, natural-language filters, anomaly flags | Missing | | cross-cutting |
| 3 | Command palette, presence, offline | Missing | | web shell and backend |

## Recommended roadmap

Close the Tier 1 gaps first, cheapest inside `./sdui` before anything that needs backend work, then add Tier 2 features that reuse the same machinery.

1. **Attachments and notes panel on detail pages** (sdui, reuses the existing attachments node; needs the attachment API to be reachable).
2. **Loading and error states** and consistent success or failure messages after save and delete (sdui only).
3. **Server-side full export and bulk actions beyond delete** (sdui action definitions plus one backend endpoint each).
4. **Foreign-key labels in list columns** (backend list API returns a label beside each id; sdui reads it).
5. **Saved views** per user and shared, pinned to workspaces (needs a small persistence API, then sdui UI).
6. **Accessibility audit** against WCAG 2.1 AA, with fixes in generated markup (sdui and web shell).
7. **Second language** in the message table, proving the localization path end to end (sdui plus translation content).
8. **Tier 2 views**: group-by, kanban for status fields, side-panel quick view, duplicate record.
9. **Tier 3 pilot**: natural-language filter on one list, where the model turns text into the existing filter syntax (cross-cutting; start only after saved views exist).

## Sources

Claims come from search-result excerpts of these vendor and community pages, retrieved 28 Sep 2026; the pages were not opened in full, so treat details as unconfirmed until checked there.

- [SAP Fiori list report floorplan](https://experience.sap.com/fiori-design-web/list-report-floorplan-sap-fiori-element/) and [usage](https://www.sap.com/design-system/fiori-design-web/v1-71/page-types/floorplans/list-report-floorplan-sap-fiori-element/usage)
- [SAP UX Q1/2025 update](https://community.sap.com/t5/technology-blog-posts-by-sap/sap-ux-q1-2025-update-part-2-sap-s-4hana-cloud-public-edition-2502-and-sap/ba-p/14015395)
- [Odoo 18: search, filter and group](https://www.odoo.com/documentation/18.0/applications/essentials/search.html) and [view architectures](https://www.odoo.com/documentation/18.0/developer/reference/user_interface/view_architectures.html)
- [Frappe Desk UI](https://frappe.io/framework/desk-ui) and [Frappe v16 highlights](https://sanskartechnolab.com/frappe-framework/frappe-framework-v16)
- [Dolibarr exports](https://wiki.dolibarr.org/index.php?title=Module_Exports_En)
- [metasfresh quick start](https://docs.metasfresh.org/webui_collection/EN/Quickstart.html)
- [Apache OFBiz headless UI](https://www.hotwaxsystems.com/hotwax-blog/apache-ofbiz-a-modern-framework-hidden-behind-a-generic-ui)
- [D365 saved views](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/dev-itpro/get-started/saved-views), [UI elements](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/fin-ops/get-started/user-interface-elements), [Copilot architecture](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/dev-itpro/copilot/copilot-architecture)
- [Business Central personalization](https://learn.microsoft.com/en-gb/dynamics365/business-central/ui-personalization-user) and [Analysis Assist](https://learn.microsoft.com/en-us/dynamics365/business-central/analysis-assist)
- [NetSuite dashboard views](https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_N496945.html) and [NetSuite Next / Ask Oracle](https://www.houseblend.io/articles/netsuite-next-ask-oracle-ai-interface)
- [Workday Canvas](https://canvas.workday.com/) and [Workday Illuminate](https://newsroom.workday.com/2024-09-17-Announcing-Workday-Illuminate-TM-The-Next-Generation-of-Workday-AI)
