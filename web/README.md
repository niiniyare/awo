# AWO ERP Frontend Development Guide

This guide explains how to build professional, data-driven user interfaces for AWO ERP modules with zero frontend coding required. By leveraging the powerful AMIS framework, you can define complex UIs entirely with JSON.

<!-- toc -->

- [AWO ERP Frontend Development Guide](#awo-erp-frontend-development-guide)
- [0. Frontend Source of Truth (Read This First)](#0-frontend-source-of-truth-read-this-first)
- [1. Core Concepts](#1-core-concepts)
  - [What You're Building](#what-youre-building)
  - [How It Works](#how-it-works)
  - [Your Workflow (3 Steps)](#your-workflow-3-steps)
- [2. Setup (5 Minutes)](#2-setup-5-minutes)
  - [Prerequisites](#prerequisites)
  - [Get AMIS SDK](#get-amis-sdk)
  - [Project Structure](#project-structure)
- [3. The Main Application Layout](#3-the-main-application-layout)
  - [The `index.html` Shell](#the-indexhtml-shell)
  - [How It Works](#how-it-works-1)
  - [Create a Dashboard Page](#create-a-dashboard-page)
  - [Test It](#test-it)
- [4. The API Contract (Critical)](#4-the-api-contract-critical)
  - [Standard API Response](#standard-api-response)
  - [Success Response (Single Object)](#success-response-single-object)
  - [Success Response (List with Pagination)](#success-response-list-with-pagination)
  - [Error Response](#error-response)
  - [Go Backend Example (with Typed Responses)](#go-backend-example-with-typed-responses)
  - [Query Parameters AMIS Sends](#query-parameters-amis-sends)
  - [CORS Configuration (Required)](#cors-configuration-required)
- [5. Building a CRUD Page](#5-building-a-crud-page)
  - [Step 1: Plan the Feature](#step-1-plan-the-feature)
  - [Step 2: Build the Go API](#step-2-build-the-go-api)
  - [Step 3: Create the JSON Schema](#step-3-create-the-json-schema)
  - [Step 4: Link in Navigation](#step-4-link-in-navigation)
  - [Step 5: Test](#step-5-test)
- [6. Common UI Patterns](#6-common-ui-patterns)
  - [Dropdown from API (Foreign Keys)](#dropdown-from-api-foreign-keys)
  - [Cascading Dropdowns](#cascading-dropdowns)
  - [Date Range Filter](#date-range-filter)
  - [File & Image Uploads](#file--image-uploads)
  - [Conditional Fields](#conditional-fields)
  - [Status Badges](#status-badges)
  - [Action Buttons with Workflow](#action-buttons-with-workflow)
- [7. Reports & Analytics](#7-reports--analytics)
  - [Table Report](#table-report)
  - [Chart Report](#chart-report)
- [8. Best Practices](#8-best-practices)
  - [Use Typed Structs for API Responses](#use-typed-structs-for-api-responses)
  - [Schema Reusability with `$ref`](#schema-reusability-with-ref)
  - [Use `initApi` for Edit Forms](#use-initapi-for-edit-forms)
  - [Keep Schemas Clean](#keep-schemas-clean)
- [9. Troubleshooting](#9-troubleshooting)
  - [Issue: Blank Page or "Loading..."](#issue-blank-page-or-loading)
  - [Issue: API Calls Failing (404, 500)](#issue-api-calls-failing-404-500)
  - [Issue: "Page not found" Error](#issue-page-not-found-error)
  - [Issue: Form Not Submitting](#issue-form-not-submitting)
  - [Issue: Dropdown Not Loading Options](#issue-dropdown-not-loading-options)
  - [Issue: Styling Looks Broken](#issue-styling-looks-broken)
- [10. Component Quick Reference](#10-component-quick-reference)

<!-- tocstop -->

## 0. Frontend Source of Truth (Read This First)

**Everything below this section describes the project's original scaffold and is not how the application is actually built, deployed, or served today.** This section reflects what's real right now; read it before acting on anything in sections 1-3 below (those are left unmodified pending a separate documentation pass — see tasks.md **R.18**).

- **Source of truth:** `web/dist/index.html` is hand-authored and hand-maintained directly. It is *not* generated from this guide's `web/pages/index.html` instructions, nor from the separate Vite project also living under `web/` (`web/index.html`, `web/package.json`, `web/vite.config.js`). That Vite project has had zero commits since the project's initial import and is now retired — see its own header comment and tasks.md **R.17**/**R.18**. Edit `web/dist/index.html` directly for any frontend change.
- **How it's actually served:** the running Go server embeds `web/dist/` and `web/showcase/` at compile time (`web/web.go`'s `//go:embed`) and serves them itself. There is no separate static-file server, no separate port, and no manual AMIS SDK download. The "Prerequisites" / "Get AMIS SDK" / "Project Structure" instructions in section 2 below describe the original scaffold's intended deployment model, which this project does not actually use — `web/sdk/`'s vendored bundle sits unused today (the served page loads AMIS from a CDN; see the version note below).
- **Supported build/validate command:** there is no compiler between this file and the browser, so "building" means validating it. Either of these is the supported command, run from `web/`:
  ```bash
  npm run validate                    # from web/ — runs the same file, via package.json
  node form-navigation.test.cjs       # equivalent, direct
  ```
  (From the repo root: `node web/form-navigation.test.cjs`.) This runs the regression tests plus a whole-file syntax check. `npm run build` / `pnpm run build` inside `web/` is deliberately blocked (see `web/package.json`). That script belongs to the retired Vite scaffold; running it would silently overwrite this file with a stale, pre-fix version if it weren't guarded.
- **Asset generation:** none. There is nothing to generate — the served file is the source file.
- **AMIS version:** `amis@6.7.0`, loaded from `unpkg.com`, is the **ratified, supported runtime version** — recorded as **ADR-026** (`docs/00-overview/DECISION_REGISTER.md`). It has been present since this project's very first commit and is the de facto, long-standing, working version. Two other version references exist, both retired and non-authoritative: `web/pages/index.html` (`3.6.6`) and the vendored `web/sdk/` bundle (unused, actually `6.13.0`). `docs/10-sdui/AMIS_RENDERER_SPEC.md` §9 previously stated the renderer is "pinned to the SDK version in `web/sdk/`" — that has been corrected to describe the real, served `6.7.0`. See tasks.md **R.18** and ADR-026 for the full evidence trail.
- **Testing workflow:** `web/form-navigation.test.cjs` is the only frontend test file. Run it via `npm run validate` (from `web/`) or directly with plain Node (`node web/form-navigation.test.cjs`, from the repo root) — no test framework or new dependency required. It unit-tests the key functions extracted live from `web/dist/index.html` and syntax-checks the whole inline `<script>` block.

---

## 1. Core Concepts

### What You're Building

AWO ERP's frontend is a **single-page application (SPA)** where you, the backend developer, can build a complete user interface without writing any HTML, CSS, or JavaScript.

- You write simple JSON files to define pages.
- The AMIS framework automatically renders professional UI components.
- Your Go backend provides data through standard REST APIs.

**The process:**

```
Backend API (Go) → JSON Schema → AMIS Engine → Professional UI
```

You get a rich, interactive UI with tables, forms, charts, and more, just by describing it in JSON.

### How It Works

The application consists of a single HTML file (`index.html`) that acts as a shell. This shell contains the navigation menu and a content area. When a user clicks a menu item:

1.  The URL hash changes (e.g., `#hrm/employees`).
2.  A small piece of JavaScript reads the hash.
3.  It fetches the corresponding JSON schema file (e.g., `schemas/pages/hrm/employees.json`).
4.  AMIS renders the content of that JSON file into the main content area.
5.  The rendered components then call your Go APIs to get or submit data.

### Your Workflow (3 Steps)

For any new feature, the workflow is simple:

1.  **Build the Go API:** Create the necessary REST endpoints for your feature (e.g., list, create, update, delete).
2.  **Write the JSON Schema:** Create a `.json` file that defines the UI for your feature using AMIS components.
3.  **Add to Navigation:** Add a link to the new page in the `index.html` sidebar.

That's it. The UI is now live.

---

## 2. Setup (5 Minutes)

### Prerequisites

You only need a simple web server to serve the static files (`index.html`, `sdk/`, `schemas/`).

```bash
# On your development machine or Termux
# If you don't have a simple server, you can use Python's
python3 -m http.server 3000

# Or with Node.js
npm install -g http-server
http-server -p 3000
```

### Get AMIS SDK

The AMIS SDK contains the core JavaScript and CSS for rendering the UI. This is a one-time setup.

```bash
# Navigate to your project's web directory
cd /path/to/your/project/web

# Create the sdk directory
mkdir -p sdk

# Download and extract the SDK
wget https://github.com/baidu/amis/releases/latest/download/sdk.tar.gz
tar -xzf sdk.tar.gz -C sdk
rm sdk.tar.gz
```

### Project Structure

Your frontend files should be organized as follows. Note the use of the `web` directory.

```
awo-erp/
├── backend/              # Your Go backend
└── web/                  # AWO ERP Frontend
    ├── sdk/              # AMIS SDK (don't modify)
    │   ├── sdk.css
    │   ├── sdk.js
    │   └── iconfont.css
    ├── pages/
    │   └── index.html    # Main application shell
    └── schemas/
        └── pages/        # Your JSON schemas go here
            ├── dashboard.json
            └── hrm/
                └── employees.json
```

---

## 3. The Main Application Layout

### The `index.html` Shell

This is the one and only HTML file for the entire application. It provides the sidebar navigation and the main content area where AMIS will render your pages.

Create `web/pages/index.html` and paste the following code.

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>AWO ERP</title>
  <link rel="stylesheet" href="../sdk/sdk.css">
  <link rel="stylesheet" href="../sdk/iconfont.css">
  <style>
    :root {
      --sidebar-width: 240px;
      --sidebar-bg: #2c3e50;
      --sidebar-text: #ecf0f1;
      --sidebar-highlight: #34495e;
      --sidebar-active: #3498db;
    }
    * { margin: 0; padding: 0; box-sizing: border-box; }
    html, body { height: 100%; font-family: sans-serif; }
    #app { display: flex; height: 100%; }
    #sidebar { width: var(--sidebar-width); background: var(--sidebar-bg); color: var(--sidebar-text); overflow-y: auto; flex-shrink: 0; }
    #sidebar .logo { padding: 20px; font-size: 20px; font-weight: bold; border-bottom: 1px solid var(--sidebar-highlight); }
    #sidebar .menu { list-style: none; }
    #sidebar .menu-item a { color: var(--sidebar-text); text-decoration: none; display: block; padding: 12px 20px; border-left: 3px solid transparent; }
    #sidebar .menu-item a:hover { background: var(--sidebar-highlight); }
    #sidebar .menu-item.active a { border-left-color: var(--sidebar-active); background: var(--sidebar-highlight); }
    #sidebar .menu-group { padding: 15px 20px 5px; font-size: 12px; color: #95a5a6; text-transform: uppercase; }
    #content { flex: 1; overflow: auto; }
    .app-loader, .error-page { display: flex; justify-content: center; align-items: center; height: 100%; text-align: center; }
    .error-page div { max-width: 400px; padding: 20px; background: #f8d7da; border: 1px solid #f5c6cb; border-radius: 5px; }
  </style>
</head>
<body>
  <div id="app">
    <div id="sidebar">
      <div class="logo">AWO ERP</div>
      <ul class="menu">
        <li class="menu-item"><a href="#dashboard">Dashboard</a></li>
        
        <div class="menu-group">HRM</div>
        <li class="menu-item"><a href="#hrm/employees">Employees</a></li>
        
        <!-- Add new navigation links here -->
      </ul>
    </div>
    <div id="content">
      <div class="app-loader"><h2>Loading AWO ERP...</h2></div>
    </div>
  </div>

  <script src="../sdk/sdk.js"></script>
  <script>
    (function() {
      const amis = amisRequire('amis/embed');
      const content = document.getElementById('content');
      const menuItems = document.querySelectorAll('.menu-item');
      
      // --- CONFIGURATION ---
      const API_BASE = 'http://localhost:8080/api'; // Change to your Go backend URL
      const SCHEMA_BASE = '../schemas/pages/';
      const DEFAULT_PAGE = 'dashboard';
      // ---------------------

      const fetcher = (config) => {
        if (config.url && !config.url.startsWith('http')) {
          config.url = API_BASE + config.url;
        }
        // Add any custom headers like Authorization here if needed
        return amis.env.fetcher(config);
      };

      function renderError(route, error) {
        content.innerHTML = `
          <div class="error-page">
            <div>
              <h3>Error Loading Page</h3>
              <p><strong>Route:</strong> #${route}</p>
              <p><strong>Details:</strong> ${error.message}</p>
              <p>Please check the file path and JSON syntax.</p>
              <a href="#${DEFAULT_PAGE}">Go to Dashboard</a>
            </div>
          </div>`;
      }

      function setActiveMenu(route) {
        menuItems.forEach(item => {
          const link = item.querySelector('a');
          if (link && link.getAttribute('href') === '#' + route) {
            item.classList.add('active');
          } else {
            item.classList.remove('active');
          }
        });
      }

      async function navigate() {
        const route = window.location.hash.slice(1) || DEFAULT_PAGE;
        setActiveMenu(route);
        content.innerHTML = '<div class="app-loader"><h2>Loading...</h2></div>';

        try {
          const response = await fetch(`${SCHEMA_BASE}${route}.json`);
          if (!response.ok) {
            throw new Error(`Schema file not found (HTTP ${response.status})`);
          }
          const schema = await response.json();
          
          amis.embed('#content', schema, {}, { fetcher: fetcher });

        } catch (error) {
          console.error(`Failed to load page '${route}':`, error);
          renderError(route, error);
        }
      }

      window.addEventListener('hashchange', navigate);
      navigate(); // Initial page load
    })();
  </script>
</body>
</html>
```

### How It Works

-   **No Page Reloads:** The `navigate` function is triggered on initial load and whenever the URL hash changes. It fetches the new JSON and renders it into the `#content` div *without* reloading the page.
-   **Central Config:** Key paths like `API_BASE` and `SCHEMA_BASE` are at the top for easy configuration.
-   **Improved Error Handling:** If a schema fails to load, a user-friendly error message is displayed directly on the page.
-   **Active Menu State:** The sidebar automatically highlights the currently active page.

### Create a Dashboard Page

AMIS needs a default page to show on first load. Create `web/schemas/pages/dashboard.json`:

```json
{
  "type": "page",
  "title": "AWO ERP Dashboard",
  "body": [
    {
      "type": "panel",
      "title": "Welcome to AWO ERP",
      "body": {
        "type": "tpl",
        "tpl": "<div style='padding: 20px; text-align: center;'><h2>Your ERP system is ready!</h2><p>Navigate to different modules using the sidebar menu.</p></div>"
      }
    }
  ]
}
```

### Test It

1.  **Start your Go backend** on port 8080.
2.  **Start a static file server** in the `web` directory on port 3000.
    ```bash
    cd /path/to/your/project/web
    python3 -m http.server 3000
    ```
3.  **Open your browser** to `http://localhost:3000/pages/index.html`.

You should see the dashboard. Clicking menu items will now load pages instantly without a full refresh.

---

## 4. The API Contract (Critical)

> **Corrected 2026-10-19** — the version of this section previously here described a `{status, msg, data, errors}` envelope and `gin`-based handlers. Neither matches this project: Awo uses Fiber, not gin, and its real, canonical envelope is `api/response`'s `{data, meta}` / `{error}` shape (`api/response/response.go`) — every framework handler already uses it. The old shape is not hypothetical-wrong; it is the exact shape proven (tasks.md R.14, findings register §B.15) to cause a real production bug: amis's own response normalizer reads a top-level `status` field, which collided with Awo entities that have their own business field literally named `status` (e.g. Expense's draft/submitted/approved). Do not reintroduce a top-level `status`/`msg` envelope.

Your Go backend **MUST** return JSON in the following structure for AMIS to understand it — this is enforced by the frontend's `adaptResponse` (`web/dist/index.html`), which bridges this exact shape into what amis's SDK expects.

### Standard API Response

`api/response` already defines these types — import and use them directly; do not redeclare an equivalent struct.

```go
// api/response.Success — the envelope for every successful response.
type Success struct {
	Data any   `json:"data"`
	Meta *Meta `json:"meta,omitempty"` // present on list responses
}

type Meta struct {
	Total    int64  `json:"total,omitempty"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
	HasMore  bool   `json:"has_more,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
}

// api/response.Envelope — the envelope for every error response.
type Envelope struct {
	Error Error `json:"error"`
}

type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}
```

### Success Response (Single Object)

Used for `GET /items/:id`, `POST /items`, `PUT /items/:id`.

```json
{
  "data": {
    "id": 1,
    "name": "John Doe",
    "email": "john@example.com"
  }
}
```

### Success Response (List with Pagination)

Used for `GET /items`. The `data` field must contain `items` (an array) and `total` (the total count); pagination metadata is carried separately in the top-level `meta` object, not nested inside `data`.

```json
{
  "data": {
    "items": [
      {"id": 1, "name": "John Doe"},
      {"id": 2, "name": "Jane Smith"}
    ],
    "total": 150
  },
  "meta": {
    "total": 150,
    "page": 1,
    "page_size": 20
  }
}
```

The frontend's `adaptResponse` also accepts a plain `{"data": [...], "meta": {"total": N}}` shape (a bare array, no nested `items`) and reshapes it into the `{items, total}` form amis's list widgets need — so a handler returning `response.Success{Data: records, Meta: &response.Meta{Total: total}}` works correctly without extra backend-side reshaping.

### Error Response

Used for validation failures or any other error.

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Validation failed",
    "fields": {
      "email": "A valid email is required.",
      "name": "Name must be at least 3 characters long."
    }
  }
}
```

### Go Backend Example (with Typed Responses)

Awo uses [Fiber](https://github.com/gofiber/fiber), not gin. Here is a real-shaped `List` handler (abbreviated from `api/handler/crud.go`):

```go
import (
	"github.com/gofiber/fiber/v2"
	"awo.so/awo/api/response"
)

// GET /api/v1/business/employees
func (h *EntityHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20) // NOT "perPage" — see the table below

	records, info, err := h.svc.Query(c.UserContext() /* ... */)
	if err != nil {
		return c.Status(response.HTTPStatus(err)).JSON(response.Wrap(err))
	}

	return c.JSON(response.Success{
		Data: records,
		Meta: &response.Meta{Total: info.Total, Page: page, PageSize: pageSize},
	})
}
```

### Query Parameters AMIS Sends

For list/CRUD pages, AMIS's crud component sends these query parameters by default. The frontend's `fetcher` (`web/dist/index.html`) already translates `perPage`/`$perPage` into `page_size` before the request leaves the browser — your handler never sees `perPage`.

| Parameter         | Description                      | Example         | Backend param name (`api/handler/crud.go`) |
| ------------------ | --------------------------------- | --------------- | --- |
| `page`            | Current page number              | `1`              | `page` (matches) |
| `perPage`         | Items per page                   | `20`             | translated to **`page_size`** by the frontend before the request is sent |
| `orderBy`         | Sort field name                  | `name`           | `orderBy` (matches) |
| `orderDir`        | Sort direction                   | `asc` or `desc`  | `orderDir` (matches) |
| _(filter fields)_ | Any field name becomes a filter  | `status=approved`| passed through as-is |

### CORS Configuration (Required)

Your Go backend must allow Cross-Origin Resource Sharing (CORS) from the frontend.

```go
import "github.com/gin-contrib/cors"

func main() {
    r := gin.Default()

    // Use a permissive CORS policy for development
    r.Use(cors.New(cors.Config{
        AllowOrigins:     []string{"*"}, // In production, restrict to your frontend's domain
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
    }))

    // Your API routes...
    r.Run(":8080")
}
```

---

## 5. Building a CRUD Page

This is the pattern for 90% of your work. Let's build a "Products" management page.

### Step 1: Plan the Feature

-   **Feature:** Manage Products
-   **Fields:** Name, SKU, Category, Price, Stock
-   **Actions:** Create, Read, Update, Delete, Search

### Step 2: Build the Go API

Ensure you have the following endpoints. They must follow the API contract defined above.

-   `GET /api/inventory/products`: Lists products (paginated).
-   `POST /api/inventory/products`: Creates a new product.
-   `GET /api/inventory/products/:id`: Gets a single product's details.
-   `PUT /api/inventory/products/:id`: Updates a product.
-   `DELETE /api/inventory/products/:id`: Deletes a product.
-   `GET /api/inventory/categories/options`: An endpoint for dropdowns (see Common Patterns).

### Step 3: Create the JSON Schema

Create `web/schemas/pages/inventory/products.json`. This file defines the entire UI for the products page.

```json
{
  "type": "page",
  "title": "Product Management",
  "body": {
    "type": "crud",
    "api": "/inventory/products",
    "syncLocation": false,
    "perPage": 20,

    "headerToolbar": [
      "bulkActions",
      "pagination",
      {
        "type": "export-excel",
        "label": "Export"
      }
    ],

    "filter": {
      "title": "Search",
      "body": [
        { "type": "input-text", "name": "keywords", "placeholder": "Search by name or SKU..." },
        { "type": "select", "name": "category_id", "label": "Category", "source": "/inventory/categories/options", "clearable": true }
      ]
    },

    "columns": [
      { "name": "id", "label": "ID", "sortable": true, "width": 80 },
      { "name": "name", "label": "Product Name", "sortable": true },
      { "name": "sku", "label": "SKU", "sortable": true },
      { "name": "category_name", "label": "Category" },
      { "name": "price", "label": "Price", "sortable": true, "type": "tpl", "tpl": "$${price|number:2}" },
      { "name": "stock_qty", "label": "Stock", "sortable": true },
      {
        "type": "operation",
        "label": "Actions",
        "buttons": [
          { "label": "Edit", "type": "button", "actionType": "dialog", "dialog": { "title": "Edit Product", "body": { "$ref": "schema://forms/product" } } },
          { "label": "Delete", "type": "button", "actionType": "ajax", "api": "delete:/inventory/products/${id}", "confirmText": "Delete this product?", "className": "text-danger" }
        ]
      }
    ],

    "toolbar": [
      {
        "type": "button",
        "label": "Add Product",
        "level": "primary",
        "actionType": "dialog",
        "dialog": {
          "title": "Add New Product",
          "body": {
            "type": "form",
            "api": "post:/inventory/products",
            "body": [
              { "type": "input-text", "name": "name", "label": "Product Name", "required": true },
              { "type": "input-text", "name": "sku", "label": "SKU", "required": true },
              { "type": "select", "name": "category_id", "label": "Category", "source": "/inventory/categories/options", "required": true },
              { "type": "input-number", "name": "price", "label": "Price", "prefix": "$", "precision": 2, "required": true },
              { "type": "input-number", "name": "stock_qty", "label": "Stock Quantity", "min": 0, "value": 0 }
            ]
          }
        }
      }
    ]
  }
}
```

*Note: The "Edit" button in this example uses `$ref` to reuse a form definition. See the Best Practices section for more details.*

### Step 4: Link in Navigation

Open `web/pages/index.html` and add a new item to the sidebar menu.

```html
<!-- ... inside <ul class="menu"> ... -->
<div class="menu-group">Inventory</div>
<li class="menu-item"><a href="#inventory/products">Products</a></li>
<!-- ... -->
```

### Step 5: Test

Refresh your browser. The "Products" link should appear. Click it to see your fully functional CRUD page.

---

## 6. Common UI Patterns

### Dropdown from API (Foreign Keys)

For fields like `category_id`, you need a dropdown populated from an API.

**JSON Schema:**
```json
{
  "type": "select",
  "name": "category_id",
  "label": "Category",
  "source": "/inventory/categories/options",
  "required": true
}
```

**Go Backend (`/api/inventory/categories/options`):**
The backend must return an object containing an `options` array. Each option needs a `label` (what the user sees) and a `value` (the ID that gets submitted).

```go
func GetCategoryOptions(c *gin.Context) {
    categories := db.GetAllCategories() // Fetch your categories
    
    type Option struct {
        Label string `json:"label"`
        Value int    `json:"value"`
    }
    
    options := make([]Option, len(categories))
    for i, cat := range categories {
        options[i] = Option{Label: cat.Name, Value: cat.ID}
    }
    
    c.JSON(http.StatusOK, AmisAPIResponse{
        Status: 0,
        Data: gin.H{
            "options": options,
        },
    })
}
```

### Cascading Dropdowns

To make one dropdown depend on another (e.g., States depends on Country), use AMIS's data mapping.

```json
[
  {
    "type": "select",
    "name": "country_id",
    "label": "Country",
    "source": "/api/countries/options"
  },
  {
    "type": "select",
    "name": "state_id",
    "label": "State",
    "source": "/api/states/options?country_id=${country_id}",
    "visibleOn": "this.country_id"
  }
]
```
AMIS automatically replaces `${country_id}` with the value from the first dropdown and triggers a refetch of the second dropdown's source.

### Date Range Filter

```json
{
  "type": "input-date-range",
  "name": "created_at",
  "label": "Date Range",
  "format": "YYYY-MM-DD"
}
```
Your backend will receive `?created_at[0]=YYYY-MM-DD&created_at[1]=YYYY-MM-DD`.

### File & Image Uploads

```json
// For general files
{
  "type": "input-file",
  "name": "attachment",
  "label": "Attachment",
  "receiver": "/upload/file"
}

// For images with preview and crop
{
  "type": "input-image",
  "name": "avatar",
  "label": "Avatar",
  "receiver": "/upload/image",
  "crop": true,
  "cropRatio": 1
}
```
The `receiver` is the API endpoint that handles the file upload. It must return the URL/path of the saved file in the `data.value` field.

### Conditional Fields

Use the `visibleOn` property to show a field based on the value of another.

```json
[
  {
    "type": "radios",
    "name": "delivery_type",
    "label": "Delivery Method",
    "options": ["pickup", "home_delivery"],
    "value": "pickup"
  },
  {
    "type": "textarea",
    "name": "delivery_address",
    "label": "Delivery Address",
    "visibleOn": "this.delivery_type === 'home_delivery'",
    "required": true
  }
]
```

### Status Badges

Use the `mapping` type to display statuses with colored badges.

```json
{
  "name": "status",
  "label": "Status",
  "type": "mapping",
  "map": {
    "pending": "<span class='label label-warning'>Pending</span>",
    "approved": "<span class='label label-success'>Approved</span>",
    "rejected": "<span class='label label-danger'>Rejected</span>",
    "*": "<span class='label label-default'>${status}</span>"
  }
}
```
The `*` is a wildcard for any value not explicitly mapped.

### Action Buttons with Workflow

Combine `visibleOn` with action buttons to create simple workflows.

```json
{
  "type": "operation",
  "label": "Actions",
  "buttons": [
    {
      "label": "Approve",
      "type": "button",
      "level": "success",
      "actionType": "ajax",
      "api": "post:/api/requests/${id}/approve",
      "confirmText": "Approve this request?",
      "visibleOn": "this.status === 'pending'"
    },
    {
      "label": "Reject",
      "type": "button",
      "level": "danger",
      "actionType": "ajax",
      "api": "post:/api/requests/${id}/reject",
      "confirmText": "Reject this request?",
      "visibleOn": "this.status === 'pending'"
    }
  ]
}
```

---

## 7. Reports & Analytics

Reports are just `page` schemas that use `service` components to fetch data and `chart` or `table` components to display it.

### Table Report

A report is a form that targets a `service` component to reload its data.

```json
{
  "type": "page",
  "title": "Attendance Report",
  "body": [
    {
      "type": "form",
      "mode": "inline",
      "target": "attendance_report_service", // Target the service by name
      "body": [
        { "type": "input-month", "name": "month", "label": "Month", "required": true },
        { "type": "select", "name": "department_id", "label": "Department", "source": "/api/departments/options" }
      ],
      "actions": [ { "type": "submit", "label": "Generate Report", "level": "primary" } ]
    },
    {
      "type": "service",
      "name": "attendance_report_service", // The named service
      "api": "/reports/attendance?month=${month}&department_id=${department_id}",
      "body": {
        "type": "table",
        "source": "${items}", // Data comes from the service API
        "columns": [
          { "name": "employee_name", "label": "Employee" },
          { "name": "days_present", "label": "Present" }
        ]
      }
    }
  ]
}
```

### Chart Report

Use the `chart` component and bind its `config` to the data from your service.

```json
{
  "type": "service",
  "api": "/reports/sales-trends?range=${dateRange}",
  "body": {
    "type": "chart",
    "config": {
      "xAxis": { "data": "${chart.labels}" },
      "series": [{ "data": "${chart.series}" }],
      "tooltip": { "trigger": "axis" }
    }
  }
}
```
Your API should return data in a structure that is easy to map, e.g., `{"chart": {"labels": ["Mon", "Tue"], "series": [120, 200]}}`.

---

## 8. Best Practices

### Use Typed Structs for API Responses

Instead of `gin.H{}`, use typed structs for your API responses. This makes your code self-documenting, reduces runtime errors, and improves autocompletion in your IDE.

### Schema Reusability with `$ref`

For common elements like an edit/create form, define it once and reuse it.

1.  Create a schema for the reusable part, e.g., `web/schemas/forms/product.json`:
    ```json
    {
      "type": "form",
      "body": [
        { "type": "input-text", "name": "name", "label": "Product Name", "required": true },
        { "type": "input-text", "name": "sku", "label": "SKU", "required": true },
        { "type": "select", "name": "category_id", "label": "Category", "source": "/inventory/categories/options", "required": true }
      ]
    }
    ```

2.  In your main CRUD schema, reference it using `$ref`. AMIS needs a custom protocol like `schema://` which you can handle or simply use relative paths. For simplicity, we can imagine a loader that understands this. A more practical approach is to use a build step to combine these files.

    ```json
    // In products.json
    {
      "label": "Edit",
      "actionType": "dialog",
      "dialog": {
        "title": "Edit Product",
        "body": {
          // This is a conceptual example.
          // Real implementation might require a build tool.
          "$ref": "schema://forms/product"
        }
      }
    }
    ```
    *For now, without a build step, you may have to repeat the form definition in both the "Add" and "Edit" dialogs.*

### Use `initApi` for Edit Forms

When an "Edit" dialog opens, it needs to fetch the existing data for the item. Use `initApi` on the form for this.

```json
{
  "label": "Edit",
  "actionType": "dialog",
  "dialog": {
    "title": "Edit Product",
    "body": {
      "type": "form",
      "initApi": "/inventory/products/${id}", // Fetches data when dialog opens
      "api": "put:/inventory/products/${id}", // Submits data on save
      "body": [
        // ... form fields ...
      ]
    }
  }
}
```

### Keep Schemas Clean

-   **File Organization:** Keep schemas organized by module (`hrm`, `inventory`). Separate complex pages or reusable parts into their own files.
-   **Naming:** Use clear, consistent names for files (`kebab-case.json`) and API endpoints.

---

## 9. Troubleshooting

Always have your browser's Developer Tools (F12) open. The **Console** and **Network** tabs are your best friends.

### Issue: Blank Page or "Loading..."

1.  **Console Errors:** Look for JavaScript errors in the console.
2.  **Network Tab:** Check if the `sdk.js` and `sdk.css` files loaded successfully (HTTP 200). If they are 404, your path in `index.html` is wrong.
3.  **Schema Load:** Check the Network tab for your schema file (e.g., `dashboard.json`). If it's 404, the path is wrong or the file doesn't exist. If it's 500, your file server has an issue.
4.  **JSON Syntax:** Copy the content of your schema file and paste it into a JSON validator to check for syntax errors like missing commas or brackets.

### Issue: API Calls Failing (404, 500)

1.  **Network Tab:** Find the failing API call. Check the URL, method (GET/POST), and status code.
2.  **CORS Error:** Look for a CORS error in the console. If you see one, your Go backend's CORS middleware is not configured correctly.
3.  **API URL:** Double-check the `api` property in your JSON schema. Is the path correct? Did you forget the leading `/`?
4.  **Backend Logs:** Check the logs of your Go application for any panic or error message corresponding to the failed request.

### Issue: "Page not found" Error

This is the custom error from our `index.html` script.
1.  **File Path:** The URL hash (`#hrm/employees`) must exactly match the file path (`schemas/pages/hrm/employees.json`). Check for typos.
2.  **File Existence:** Make sure the JSON file actually exists at that location.

### Issue: Form Not Submitting

1.  **`api` Property:** Ensure the `<form>` or `<crud>` component has an `api` property defined.
2.  **Validation:** Check if any fields have validation rules that are failing silently. The browser console may show details.
3.  **Network Tab:** See if an API request is even being made on submit. If not, there's an issue with the AMIS form configuration.

### Issue: Dropdown Not Loading Options

1.  **`source` Property:** Check the `source` URL on your `select` component.
2.  **Network Tab:** Find the request to the `source` URL. Did it succeed?
3.  **Response Format:** Inspect the response body. It **must** be in the format `{"status": 0, "data": {"options": [{"label": "...", "value": "..."}]}}`. A common mistake is forgetting the `options` wrapper.

### Issue: Styling Looks Broken

1.  **Network Tab:** Check that `sdk.css` and `iconfont.css` are loading correctly. A 404 error on these files is the most common cause.
2.  **Paths:** Verify the `<link>` paths in your `index.html` are correct relative to the file's location.

---

## 10. Component Quick Reference

| Component | Purpose                       |
| :-------- | :---------------------------- |
| `page`    | Root container for a page.    |
| `crud`    | All-in-one CRUD interface.    |
| `form`    | Data input and submission.    |
| `table`   | Display tabular data.         |
| `service` | Fetches data from an API.     |
| `panel`   | A card-like container.        |
| `grid`    | For creating column layouts.  |
| `tabs`    | A tabbed interface.           |
| `wizard`  | A multi-step form.            |
| `dialog`  | A modal popup.                |
| `drawer`  | A slide-out side panel.       |
| `chart`   | For data visualization.       |
| `mapping` | To map values to display text.|
| `tpl`     | For rendering simple HTML.    |