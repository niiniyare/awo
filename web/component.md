# AWO ERP Advanced UI Components & Multi-Tenant Guide
## Building Tenant-Aware, Feature-Rich ERP Interfaces

> **For**: Backend developers building multi-tenant AWO ERP systems  
> **What**: Advanced UI components, multi-tenant patterns, RBAC/ABAC integration  
> **Focus**: Feature flags, tenant customization, localization, and identity-driven UIs

---

## Table of Contents

### Part 1: Advanced UI Components
1. [Select with "Add New" Feature](#select-with-add-new-feature)
2. [Advanced Table Components](#advanced-table-components)
3. [Inline Editing](#inline-editing)
4. [Quick Edit Drawer](#quick-edit-drawer)
5. [Auto-Complete & Type-Ahead](#auto-complete--type-ahead)
6. [Multi-Level Cascading Selects](#multi-level-cascading-selects)
7. [Tree Select](#tree-select)
8. [Transfer (Dual List)](#transfer-dual-list)
9. [Input Groups & Combinations](#input-groups--combinations)
10. [Custom Validators](#custom-validators)

### Part 2: Multi-Tenant Architecture
11. [Multi-Tenant Overview](#multi-tenant-overview)
12. [Tenant Context & Session](#tenant-context--session)
13. [Feature Flags System](#feature-flags-system)
14. [Configuration-Driven UI](#configuration-driven-ui)
15. [Identity Service Integration](#identity-service-integration)
16. [RBAC Implementation](#rbac-implementation)
17. [ABAC Implementation](#abac-implementation)

### Part 3: Tenant Customization
18. [Localization (i18n)](#localization-i18n)
19. [Date/Time Formats](#datetime-formats)
20. [Currency & Number Formats](#currency--number-formats)
21. [Tenant Branding](#tenant-branding)
22. [Custom Fields](#custom-fields)
23. [Workflow Customization](#workflow-customization)

### Part 4: Complete Examples
24. [Complete Multi-Tenant CRUD](#complete-multi-tenant-crud)
25. [Dynamic Form Based on Permissions](#dynamic-form-based-on-permissions)
26. [Tenant-Aware Dashboard](#tenant-aware-dashboard)
27. [Best Practices](#best-practices)

---

## Select with "Add New" Feature

One of the most useful ERP features: add new options directly from a dropdown.

### Basic Implementation

**Frontend Schema:**
```json
{
  "type": "select",
  "name": "department_id",
  "label": "Department",
  "source": "/hrm/departments/options",
  "clearable": true,
  "searchable": true,
  "creatable": true,
  "createBtnLabel": "Add New Department",
  "addApi": "post:/hrm/departments/quick-add",
  "addControls": [
    {
      "type": "input-text",
      "name": "name",
      "label": "Department Name",
      "required": true,
      "placeholder": "Enter department name"
    },
    {
      "type": "input-text",
      "name": "code",
      "label": "Department Code",
      "required": true,
      "placeholder": "e.g., IT, HR, FIN"
    },
    {
      "type": "textarea",
      "name": "description",
      "label": "Description",
      "maxRows": 3
    }
  ]
}
```

**Go Backend:**
```go
// GET /api/hrm/departments/options
func GetDepartmentOptions(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    departments, err := db.GetDepartmentsByTenant(tenantID)
    if err != nil {
        c.JSON(500, gin.H{"status": 500, "msg": "Failed to load departments"})
        return
    }
    
    options := []map[string]any{}
    for _, dept := range departments {
        options = append(options, map[string]any{
            "label": dept.Name,
            "value": dept.ID,
        })
    }
    
    c.JSON(200, gin.H{
        "status": 0,
        "data": gin.H{
            "options": options,
        },
    })
}

// POST /api/hrm/departments/quick-add
func QuickAddDepartment(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    userID := c.GetString("user_id")
    
    var input struct {
        Name        string `json:"name" binding:"required"`
        Code        string `json:"code" binding:"required"`
        Description string `json:"description"`
    }
    
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(422, gin.H{
            "status": 422,
            "msg": "Validation failed",
            "errors": map[string]string{
                "name": "Name is required",
            },
        })
        return
    }
    
    // Check permissions
    if !hasPermission(userID, "hrm.departments.create") {
        c.JSON(403, gin.H{
            "status": 403,
            "msg": "You don't have permission to create departments",
        })
        return
    }
    
    department := &Department{
        TenantID:    tenantID,
        Name:        input.Name,
        Code:        input.Code,
        Description: input.Description,
        CreatedBy:   userID,
    }
    
    if err := db.CreateDepartment(department); err != nil {
        c.JSON(500, gin.H{
            "status": 500,
            "msg": "Failed to create department",
        })
        return
    }
    
    // Return the new option
    c.JSON(200, gin.H{
        "status": 0,
        "msg": "Department created successfully",
        "data": gin.H{
            "label": department.Name,
            "value": department.ID,
        },
    })
}
```

### Advanced: Add New with Dialog

For more complex forms:

```json
{
  "type": "select",
  "name": "customer_id",
  "label": "Customer",
  "source": "/sales/customers/options",
  "searchable": true,
  "clearable": true,
  "creatable": true,
  "createBtnLabel": "Add New Customer",
  "addDialog": {
    "title": "Create New Customer",
    "size": "lg",
    "body": {
      "type": "form",
      "api": "post:/sales/customers/quick-add",
      "body": [
        {
          "type": "radios",
          "name": "customer_type",
          "label": "Customer Type",
          "options": [
            {"label": "Individual", "value": "individual"},
            {"label": "Business", "value": "business"}
          ],
          "value": "individual",
          "required": true
        },
        {
          "type": "input-text",
          "name": "name",
          "label": "Customer Name",
          "required": true
        },
        {
          "type": "input-email",
          "name": "email",
          "label": "Email",
          "required": true
        },
        {
          "type": "input-text",
          "name": "phone",
          "label": "Phone",
          "required": true
        },
        {
          "type": "input-text",
          "name": "tax_id",
          "label": "Tax ID",
          "visibleOn": "this.customer_type === 'business'"
        },
        {
          "type": "textarea",
          "name": "address",
          "label": "Address"
        }
      ]
    }
  }
}
```

---

## Advanced Table Components

### Inline Editing

Edit records directly in the table without opening dialogs:

```json
{
  "type": "crud",
  "api": "/inventory/products",
  "syncLocation": false,
  "quickSaveApi": "put:/inventory/products/${id}",
  "quickSaveItemApi": "put:/inventory/products/${id}",
  "columns": [
    {
      "name": "id",
      "label": "ID",
      "width": 60
    },
    {
      "name": "name",
      "label": "Product Name",
      "quickEdit": {
        "type": "input-text",
        "required": true,
        "saveImmediately": true
      }
    },
    {
      "name": "sku",
      "label": "SKU",
      "quickEdit": {
        "type": "input-text",
        "saveImmediately": true
      }
    },
    {
      "name": "category_id",
      "label": "Category",
      "type": "mapping",
      "map": {
        "1": "Electronics",
        "2": "Furniture",
        "3": "Office Supplies"
      },
      "quickEdit": {
        "type": "select",
        "source": "/inventory/categories/options",
        "saveImmediately": true
      }
    },
    {
      "name": "price",
      "label": "Price",
      "type": "tpl",
      "tpl": "KES ${price|number:2}",
      "quickEdit": {
        "type": "input-number",
        "prefix": "KES",
        "precision": 2,
        "min": 0,
        "saveImmediately": true
      }
    },
    {
      "name": "stock_qty",
      "label": "Stock",
      "quickEdit": {
        "type": "input-number",
        "min": 0,
        "saveImmediately": true
      }
    },
    {
      "name": "is_active",
      "label": "Status",
      "type": "mapping",
      "map": {
        "1": "<span class='label label-success'>Active</span>",
        "0": "<span class='label label-danger'>Inactive</span>"
      },
      "quickEdit": {
        "type": "switch",
        "trueValue": 1,
        "falseValue": 0,
        "saveImmediately": true
      }
    }
  ]
}
```

### Expandable Rows

Show additional details when row is expanded:

```json
{
  "type": "crud",
  "api": "/sales/orders",
  "expandable": true,
  "columns": [
    {"name": "order_number", "label": "Order #"},
    {"name": "customer_name", "label": "Customer"},
    {"name": "total", "label": "Total", "type": "tpl", "tpl": "KES ${total|number:2}"},
    {"name": "status", "label": "Status"}
  ],
  "itemActions": [
    {
      "type": "button",
      "label": "Details",
      "level": "link",
      "actionType": "expand"
    }
  ],
  "expandedRowRender": {
    "type": "service",
    "api": "/sales/orders/${id}/items",
    "body": {
      "type": "table",
      "columns": [
        {"name": "product_name", "label": "Product"},
        {"name": "quantity", "label": "Qty"},
        {"name": "unit_price", "label": "Price", "type": "tpl", "tpl": "KES ${unit_price|number:2}"},
        {"name": "subtotal", "label": "Subtotal", "type": "tpl", "tpl": "KES ${subtotal|number:2}"}
      ]
    }
  }
}
```

### Row Actions Menu

Dropdown menu for actions:

```json
{
  "type": "operation",
  "label": "Actions",
  "buttons": [
    {
      "type": "dropdown-button",
      "label": "More",
      "level": "link",
      "buttons": [
        {
          "type": "button",
          "label": "View",
          "icon": "fa fa-eye",
          "actionType": "drawer",
          "drawer": {
            "title": "Order Details",
            "body": {
              "type": "service",
              "api": "/sales/orders/${id}",
              "body": "..."
            }
          }
        },
        {
          "type": "button",
          "label": "Print",
          "icon": "fa fa-print",
          "actionType": "ajax",
          "api": "post:/sales/orders/${id}/print"
        },
        {
          "type": "button",
          "label": "Send Email",
          "icon": "fa fa-envelope",
          "actionType": "dialog",
          "dialog": {
            "title": "Send Order Email",
            "body": {
              "type": "form",
              "api": "post:/sales/orders/${id}/send-email",
              "body": [
                {
                  "type": "input-email",
                  "name": "to",
                  "label": "To",
                  "value": "${customer_email}",
                  "required": true
                },
                {
                  "type": "textarea",
                  "name": "message",
                  "label": "Message"
                }
              ]
            }
          }
        },
        {
          "type": "divider"
        },
        {
          "type": "button",
          "label": "Cancel Order",
          "icon": "fa fa-times",
          "level": "danger",
          "actionType": "ajax",
          "api": "post:/sales/orders/${id}/cancel",
          "confirmText": "Are you sure you want to cancel this order?",
          "visibleOn": "this.status !== 'cancelled'"
        }
      ]
    }
  ]
}
```

### Bulk Actions with Conditions

```json
{
  "type": "crud",
  "api": "/sales/invoices",
  "bulkActions": [
    {
      "label": "Mark as Paid",
      "actionType": "ajax",
      "api": "post:/sales/invoices/bulk-paid",
      "confirmText": "Mark selected invoices as paid?",
      "disabledOn": "!this.items || this.items.filter(item => item.status !== 'unpaid').length > 0"
    },
    {
      "label": "Send Reminders",
      "actionType": "ajax",
      "api": "post:/sales/invoices/bulk-remind",
      "confirmText": "Send payment reminders?"
    },
    {
      "label": "Export Selected",
      "actionType": "ajax",
      "api": "post:/sales/invoices/bulk-export"
    },
    {
      "type": "divider"
    },
    {
      "label": "Delete",
      "level": "danger",
      "actionType": "ajax",
      "api": "delete:/sales/invoices/batch",
      "confirmText": "Delete selected invoices?"
    }
  ]
}
```

---

## Quick Edit Drawer

Edit records in a side panel without leaving the list:

```json
{
  "type": "crud",
  "api": "/hrm/employees",
  "columns": [
    {"name": "name", "label": "Name"},
    {"name": "email", "label": "Email"},
    {"name": "department_name", "label": "Department"},
    {
      "type": "operation",
      "label": "Actions",
      "buttons": [
        {
          "label": "Quick Edit",
          "type": "button",
          "level": "link",
          "icon": "fa fa-edit",
          "actionType": "drawer",
          "drawer": {
            "title": "Edit ${name}",
            "position": "right",
            "size": "md",
            "closeOnEsc": true,
            "body": {
              "type": "form",
              "api": "put:/hrm/employees/${id}",
              "initApi": "/hrm/employees/${id}",
              "body": [
                {
                  "type": "input-text",
                  "name": "name",
                  "label": "Full Name",
                  "required": true
                },
                {
                  "type": "input-email",
                  "name": "email",
                  "label": "Email",
                  "required": true
                },
                {
                  "type": "select",
                  "name": "department_id",
                  "label": "Department",
                  "source": "/hrm/departments/options",
                  "required": true
                },
                {
                  "type": "input-text",
                  "name": "phone",
                  "label": "Phone"
                },
                {
                  "type": "switch",
                  "name": "is_active",
                  "label": "Active",
                  "trueValue": 1,
                  "falseValue": 0
                }
              ]
            }
          }
        }
      ]
    }
  ]
}
```

---

## Auto-Complete & Type-Ahead

Search as you type with suggestions:

```json
{
  "type": "input-text",
  "name": "customer_name",
  "label": "Customer",
  "required": true,
  "autoComplete": "/sales/customers/search?q=${term}",
  "placeholder": "Start typing customer name..."
}
```

**Backend:**
```go
func SearchCustomers(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    query := c.Query("q")
    
    if len(query) < 2 {
        c.JSON(200, gin.H{
            "status": 0,
            "data": gin.H{
                "options": []any{},
            },
        })
        return
    }
    
    customers, _ := db.SearchCustomers(tenantID, query, 10)
    
    options := []map[string]any{}
    for _, customer := range customers {
        options = append(options, map[string]any{
            "label": customer.Name,
            "value": customer.Name,
            "id":    customer.ID,
        })
    }
    
    c.JSON(200, gin.H{
        "status": 0,
        "data": gin.H{
            "options": options,
        },
    })
}
```

### Advanced: Select with Search API

```json
{
  "type": "select",
  "name": "product_id",
  "label": "Product",
  "searchable": true,
  "source": {
    "method": "get",
    "url": "/inventory/products/search",
    "sendOn": "this.keywords && this.keywords.length >= 2",
    "data": {
      "keywords": "${keywords}"
    }
  },
  "labelField": "name",
  "valueField": "id",
  "placeholder": "Type to search products..."
}
```

---

## Multi-Level Cascading Selects

Country → State → City → Area:

```json
[
  {
    "type": "select",
    "name": "country_id",
    "label": "Country",
    "source": "/settings/countries/options",
    "required": true
  },
  {
    "type": "select",
    "name": "state_id",
    "label": "State/Province",
    "source": "/settings/states/options?country_id=${country_id}",
    "required": true,
    "visibleOn": "this.country_id",
    "clearValueOnHidden": true
  },
  {
    "type": "select",
    "name": "city_id",
    "label": "City",
    "source": "/settings/cities/options?state_id=${state_id}",
    "visibleOn": "this.state_id",
    "clearValueOnHidden": true
  },
  {
    "type": "select",
    "name": "area_id",
    "label": "Area",
    "source": "/settings/areas/options?city_id=${city_id}",
    "visibleOn": "this.city_id",
    "clearValueOnHidden": true
  }
]
```

---

## Tree Select

For hierarchical data (departments, categories, locations):

```json
{
  "type": "tree-select",
  "name": "category_id",
  "label": "Category",
  "source": "/inventory/categories/tree",
  "required": true,
  "searchable": true,
  "multiple": false,
  "cascade": true,
  "withChildren": false,
  "onlyChildren": true
}
```

**Backend Response:**
```go
func GetCategoryTree(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    categories := db.GetCategoriesByTenant(tenantID)
    
    // Build tree structure
    tree := buildTree(categories, 0)
    
    c.JSON(200, gin.H{
        "status": 0,
        "data": gin.H{
            "options": tree,
        },
    })
}

func buildTree(categories []Category, parentID int) []map[string]any {
    result := []map[string]any{}
    
    for _, cat := range categories {
        if cat.ParentID == parentID {
            node := map[string]any{
                "label":    cat.Name,
                "value":    cat.ID,
                "children": buildTree(categories, cat.ID),
            }
            result = append(result, node)
        }
    }
    
    return result
}
```

**Response Format:**
```json
{
  "status": 0,
  "data": {
    "options": [
      {
        "label": "Electronics",
        "value": 1,
        "children": [
          {
            "label": "Computers",
            "value": 2,
            "children": [
              {"label": "Laptops", "value": 3},
              {"label": "Desktops", "value": 4}
            ]
          },
          {
            "label": "Mobile Phones",
            "value": 5
          }
        ]
      }
    ]
  }
}
```

---

## Transfer (Dual List)

For selecting multiple items from a large list:

```json
{
  "type": "transfer",
  "name": "role_permissions",
  "label": "Permissions",
  "source": "/settings/permissions/all",
  "searchable": true,
  "selectMode": "tree",
  "sortable": true
}
```

**Backend:**
```go
func GetAllPermissions(c *gin.Context) {
    permissions := []map[string]any{
        {"label": "HRM", "value": "hrm", "children": []map[string]any{
            {"label": "View Employees", "value": "hrm.employees.view"},
            {"label": "Create Employees", "value": "hrm.employees.create"},
            {"label": "Edit Employees", "value": "hrm.employees.edit"},
            {"label": "Delete Employees", "value": "hrm.employees.delete"},
        }},
        {"label": "Inventory", "value": "inventory", "children": []map[string]any{
            {"label": "View Products", "value": "inventory.products.view"},
            {"label": "Manage Stock", "value": "inventory.stock.manage"},
        }},
    }
    
    c.JSON(200, gin.H{
        "status": 0,
        "data": gin.H{
            "options": permissions,
        },
    })
}
```

---

## Input Groups & Combinations

### Input Group

```json
{
  "type": "input-group",
  "label": "Price Range",
  "body": [
    {
      "type": "input-number",
      "name": "min_price",
      "placeholder": "Min",
      "prefix": "KES",
      "precision": 2
    },
    {
      "type": "static",
      "value": "to"
    },
    {
      "type": "input-number",
      "name": "max_price",
      "placeholder": "Max",
      "prefix": "KES",
      "precision": 2
    }
  ]
}
```

### Combo (Repeatable Fields)

```json
{
  "type": "combo",
  "name": "phone_numbers",
  "label": "Phone Numbers",
  "multiple": true,
  "multiLine": true,
  "minLength": 1,
  "maxLength": 5,
  "addButtonText": "Add Phone",
  "items": [
    {
      "type": "select",
      "name": "type",
      "label": "Type",
      "options": [
        {"label": "Mobile", "value": "mobile"},
        {"label": "Office", "value": "office"},
        {"label": "Home", "value": "home"}
      ],
      "required": true,
      "columnClassName": "col-sm-4"
    },
    {
      "type": "input-text",
      "name": "number",
      "label": "Number",
      "required": true,
      "placeholder": "+254 700 000000",
      "columnClassName": "col-sm-8"
    }
  ]
}
```

---

## Custom Validators

```json
{
  "type": "input-text",
  "name": "tax_id",
  "label": "Tax ID",
  "required": true,
  "validations": {
    "isLength": {
      "min": 10,
      "max": 10
    },
    "matchRegexp": "/^[A-Z0-9]+$/"
  },
  "validationErrors": {
    "isLength": "Tax ID must be exactly 10 characters",
    "matchRegexp": "Tax ID must contain only uppercase letters and numbers"
  }
}
```

### Remote Validation

```json
{
  "type": "input-email",
  "name": "email",
  "label": "Email",
  "required": true,
  "validateApi": "/users/validate-email?email=${email}",
  "validateOnChange": true
}
```

**Backend:**
```go
func ValidateEmail(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    email := c.Query("email")
    
    exists := db.EmailExists(tenantID, email)
    
    if exists {
        c.JSON(200, gin.H{
            "status": 422,
            "msg": "Email already in use",
        })
        return
    }
    
    c.JSON(200, gin.H{
        "status": 0,
        "msg": "Email is available",
    })
}
```

---

## Multi-Tenant Overview

### Architecture

```
┌─────────────────────────────────────────────────┐
│                 Frontend (AMIS)                  │
│  - Loads tenant context from session/token      │
│  - Passes tenant_id in API calls                │
│  - Adapts UI based on feature flags             │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│              Go Backend API                      │
│  - Middleware extracts tenant context           │
│  - Feature flag checks                          │
│  - Permission checks (RBAC/ABAC)                │
│  - Apply tenant-specific config                 │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│              PostgreSQL (RLS)                    │
│  - Row-level security by tenant_id              │
│  - Tenant-isolated data                         │
│  - Shared schema, isolated data                 │
└─────────────────────────────────────────────────┘
```

### Session Structure

**Go Backend Session/JWT:**
```go
type SessionContext struct {
    TenantID     string   `json:"tenant_id"`
    UserID       string   `json:"user_id"`
    Username     string   `json:"username"`
    Email        string   `json:"email"`
    Roles        []string `json:"roles"`
    Permissions  []string `json:"permissions"`
    FeatureFlags []string `json:"feature_flags"`
    Locale       string   `json:"locale"`
    TimeZone     string   `json:"timezone"`
    DateFormat   string   `json:"date_format"`
    Currency     string   `json:"currency"`
}
```

---

## Tenant Context & Session

### Backend Middleware

```go
// middleware/tenant.go
func TenantContextMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract from JWT or session
        claims := extractJWTClaims(c)
        
        // Set tenant context
        c.Set("tenant_id", claims.TenantID)
        c.Set("user_id", claims.UserID)
        c.Set("username", claims.Username)
        c.Set("roles", claims.Roles)
        c.Set("permissions", claims.Permissions)
        c.Set("feature_flags", claims.FeatureFlags)
        c.Set("locale", claims.Locale)
        c.Set("timezone", claims.TimeZone)
        c.Set("date_format", claims.DateFormat)
        c.Set("currency", claims.Currency)
        
        // Set PostgreSQL RLS context
        db := getDB()
        db.Exec("SET app.tenant_id = ?", claims.TenantID)
        db.Exec("SET app.user_id = ?", claims.UserID)
        
        c.Next()
    }
}
```

### Frontend Context Endpoint

```go
// GET /api/context
func GetUserContext(c *gin.Context) {
    context := gin.H{
        "tenant_id":     c.GetString("tenant_id"),
        "user_id":       c.GetString("user_id"),
        "username":      c.GetString("username"),
        "email":         c.GetString("email"),
        "roles":         c.GetStringSlice("roles"),
        "permissions":   c.GetStringSlice("permissions"),
        "feature_flags": c.GetStringSlice("feature_flags"),
        "locale":        c.GetString("locale"),
        "timezone":      c.GetString("timezone"),
        "date_format":   c.GetString("date_format"),
        "currency":      c.GetString("currency"),
    }
    
    c.JSON(200, gin.H{
        "status": 0,
        "data":   context,
    })
}
```

### Frontend: Load Context on Init

Update `pages/index.html`:

```html
<script>
(function() {
  const amis = amisRequire('amis/embed');
  const API_BASE = 'http://localhost:8080/api';
  
  // Global context
  let userContext = null;
  
  const fetcher = (config) => {
    if (config.url && !config.url.startsWith('http')) {
      config.url = API_BASE + config.url;
    }
    return amis.env.fetcher(config);
  };
  
  // Load user context first
  fetch(API_BASE + '/context', {
    credentials: 'include'
  })
    .then(res => res.json())
    .then(response => {
      userContext = response.data;
      
      // Store globally for schema access
      window.userContext = userContext;
      
      // Now load the page
      navigate();
    })
    .catch(error => {
      console.error('Failed to load context:', error);
      window.location.href = '/login.html';
    });
  
  function navigate() {
    const route = window.location.hash.slice(1) || 'dashboard';
    
    fetch(`../schemas/pages/${route}.json`)
      .then(response => response.json())
      .then(schema => {
        // Inject context into schema data
        schema.data = schema.data || {};
        schema.data.context = userContext;
        
        amis.embed('#content', schema, {}, {
          fetcher: fetcher
        });
      });
  }
  
  window.addEventListener('hashchange', navigate);
})();
</script>
```

---

## Feature Flags System

### Backend: Feature Flag Service

```go
// internal/features/service.go
type FeatureFlag struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Enabled     bool      `json:"enabled"`
    TenantID    string    `json:"tenant_id"`
}

type FeatureService struct {
    db *gorm.DB
}

func (s *FeatureService) IsEnabled(tenantID, featureID string) bool {
    var flag FeatureFlag
    err := s.db.Where("id = ? AND tenant_id = ?", featureID, tenantID).First(&flag).Error
    if err != nil {
        // Check global default
        err = s.db.Where("id = ? AND tenant_id IS NULL", featureID).First(&flag).Error
        if err != nil {
            return false
        }
    }
    return flag.Enabled
}

func (s *FeatureService) GetTenantFeatures(tenantID string) []string {
    var flags []FeatureFlag
    s.db.Where("tenant_id = ? AND enabled = ?", tenantID, true).Find(&flags)
    
    // Also get global enabled features
    var globalFlags []FeatureFlag
    s.db.Where("tenant_id IS NULL AND enabled = ?", true).Find(&globalFlags)
    
    features := []string{}
    for _, flag := range flags {
        features = append(features, flag.ID)
    }
    for _, flag := range globalFlags {
        features = append(features, flag.ID)
    }
    
    return features
}
```

### Middleware: Feature Check

```go
func RequireFeature(featureID string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetString("tenant_id")
        features := c.GetStringSlice("feature_flags")
        
        enabled := false
        for _, f := range features {
            if f == featureID {
                enabled = true
                break
            }
        }
        
        if !enabled {
            c.JSON(403, gin.H{
                "status": 403,
                "msg": fmt.Sprintf("Feature '%s' is not enabled for your organization", featureID),
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// Usage
hrm.GET("/payroll", RequireFeature("hrm.payroll"), ListPayroll)
```

### Frontend: Feature-Based UI

**Schema with Feature Checks:**
```json
{
  "type": "page",
  "title": "HRM Dashboard",
  "body": [
    {
      "type": "panel",
      "title": "Employees",
      "body": "..."
    },
    {
      "type": "panel",
      "title": "Payroll",
      "body": "...",
      "visibleOn": "${context.feature_flags && context.feature_flags.indexOf('hrm.payroll') >= 0}"
    },
    {
      "type": "panel",
      "title": "Time Tracking",
      "body": "...",
      "visibleOn": "${context.feature_flags && context.feature_flags.indexOf('hrm.time_tracking') >= 0}"
    },
    {
      "type": "panel",
      "title": "Advanced Analytics",
      "body": "...",
      "visibleOn": "${context.feature_flags && context.feature_flags.indexOf('analytics.advanced') >= 0}"
    }
  ]
}
```

### Dynamic Navigation Based on Features

```json
{
  "type": "service",
  "api": "/navigation/menu",
  "body": {
    "type": "nav",
    "stacked": true,
    "source": "${menu_items}"
  }
}
```

**Backend:**
```go
func GetNavigationMenu(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    features := c.GetStringSlice("feature_flags")
    permissions := c.GetStringSlice("permissions")
    
    menuItems := []map[string]any{}
    
    // Dashboard (always visible)
    menuItems = append(menuItems, map[string]any{
        "label": "Dashboard",
        "to":    "#dashboard",
        "icon":  "fa fa-home",
    })
    
    // HRM Menu
    if hasPermission(permissions, "hrm.access") {
        hrmItems := []map[string]any{
            {"label": "Employees", "to": "#hrm/employees"},
            {"label": "Attendance", "to": "#hrm/attendance"},
        }
        
        if hasFeature(features, "hrm.payroll") {
            hrmItems = append(hrmItems, map[string]any{
                "label": "Payroll",
                "to":    "#hrm/payroll",
            })
        }
        
        if hasFeature(features, "hrm.time_tracking") {
            hrmItems = append(hrmItems, map[string]any{
                "label": "Time Tracking",
                "to":    "#hrm/time-tracking",
            })
        }
        
        menuItems = append(menuItems, map[string]any{
            "label":    "HRM",
            "icon":     "fa fa-users",
            "children": hrmItems,
        })
    }
    
    // Inventory Menu
    if hasPermission(permissions, "inventory.access") {
        invItems := []map[string]any{
            {"label": "Products", "to": "#inventory/products"},
            {"label": "Stock", "to": "#inventory/stock"},
        }
        
        if hasFeature(features, "inventory.barcode") {
            invItems = append(invItems, map[string]any{
                "label": "Barcode Scanner",
                "to":    "#inventory/barcode",
            })
        }
        
        menuItems = append(menuItems, map[string]any{
            "label":    "Inventory",
            "icon":     "fa fa-boxes",
            "children": invItems,
        })
    }
    
    c.JSON(200, gin.H{
        "status": 0,
        "data": gin.H{
            "menu_items": menuItems,
        },
    })
}
```

---

## Configuration-Driven UI

### Tenant Configuration

```go
type TenantConfig struct {
    TenantID string                 `json:"tenant_id"`
    Config   map[string]any `json:"config"`
}

// Example config
{
  "hrm": {
    "employee_id_format": "EMP-{YYYY}-{0000}",
    "probation_period_days": 90,
    "require_approval_chain": true,
    "max_leave_days": 21
  },
  "inventory": {
    "enable_batch_tracking": true,
    "enable_serial_tracking": false,
    "low_stock_threshold": 10
  },
  "sales": {
    "require_customer_approval": true,
    "auto_invoice_generation": true,
    "payment_terms_days": 30
  },
  "ui": {
    "items_per_page": 20,
    "date_format": "DD/MM/YYYY",
    "time_format": "24h",
    "first_day_of_week": "monday"
  }
}
```

### Load Configuration

```go
func GetTenantConfig(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    var config TenantConfig
    db.Where("tenant_id = ?", tenantID).First(&config)
    
    c.JSON(200, gin.H{
        "status": 0,
        "data":   config.Config,
    })
}
```

### Use Configuration in Schemas

```json
{
  "type": "crud",
  "api": "/hrm/employees",
  "perPage": "${context.config.ui.items_per_page || 20}",
  "columns": [
    {
      "name": "hire_date",
      "label": "Hire Date",
      "type": "date",
      "format": "${context.config.ui.date_format || 'YYYY-MM-DD'}"
    }
  ]
}
```

### Conditional Workflow

```json
{
  "type": "form",
  "api": "post:/hrm/leave-requests",
  "body": [
    {
      "type": "input-date-range",
      "name": "dates",
      "label": "Leave Dates",
      "required": true,
      "maxDays": "${context.config.hrm.max_leave_days}"
    },
    {
      "type": "select",
      "name": "approver_id",
      "label": "Approver",
      "source": "/hrm/approvers/options",
      "required": true,
      "visibleOn": "${context.config.hrm.require_approval_chain}"
    }
  ]
}
```

---

## Identity Service Integration

### Authentication Flow

```go
// POST /api/auth/login
func Login(c *gin.Context) {
    var input struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(422, gin.H{"status": 422, "msg": "Invalid input"})
        return
    }
    
    // Authenticate with identity service
    user, err := identityService.Authenticate(input.Email, input.Password)
    if err != nil {
        c.JSON(401, gin.H{"status": 401, "msg": "Invalid credentials"})
        return
    }
    
    // Get tenant
    tenant, _ := db.GetUserTenant(user.ID)
    
    // Get roles and permissions
    roles := identityService.GetUserRoles(user.ID, tenant.ID)
    permissions := identityService.GetUserPermissions(user.ID, tenant.ID)
    
    // Get feature flags
    features := featureService.GetTenantFeatures(tenant.ID)
    
    // Get tenant config
    config := configService.GetTenantConfig(tenant.ID)
    
    // Create JWT
    token := createJWT(SessionContext{
        TenantID:     tenant.ID,
        UserID:       user.ID,
        Username:     user.Username,
        Email:        user.Email,
        Roles:        roles,
        Permissions:  permissions,
        FeatureFlags: features,
        Locale:       config.Locale,
        TimeZone:     config.TimeZone,
        DateFormat:   config.DateFormat,
        Currency:     config.Currency,
    })
    
    c.JSON(200, gin.H{
        "status": 0,
        "data": gin.H{
            "token": token,
            "user":  user,
        },
    })
}
```

---

## RBAC Implementation

### Role-Based Access Control

**Database Schema:**
```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    permissions JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL,
    role_id UUID NOT NULL REFERENCES roles(id),
    tenant_id UUID NOT NULL,
    PRIMARY KEY (user_id, role_id, tenant_id)
);
```

### Permission Check Middleware

```go
func RequirePermission(permission string) gin.HandlerFunc {
    return func(c *gin.Context) {
        permissions := c.GetStringSlice("permissions")
        
        hasPermission := false
        for _, p := range permissions {
            if p == permission || p == "*" {
                hasPermission = true
                break
            }
            
            // Check wildcard patterns
            if strings.HasSuffix(p, ".*") {
                prefix := strings.TrimSuffix(p, ".*")
                if strings.HasPrefix(permission, prefix) {
                    hasPermission = true
                    break
                }
            }
        }
        
        if !hasPermission {
            c.JSON(403, gin.H{
                "status": 403,
                "msg":    "You don't have permission to perform this action",
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// Usage
api.PUT("/hrm/employees/:id", 
    RequirePermission("hrm.employees.edit"),
    UpdateEmployee)
```

### Frontend: Permission-Based UI

```json
{
  "type": "page",
  "title": "Employees",
  "toolbar": [
    {
      "type": "button",
      "label": "Add Employee",
      "level": "primary",
      "actionType": "dialog",
      "visibleOn": "${context.permissions && context.permissions.indexOf('hrm.employees.create') >= 0}",
      "dialog": "..."
    },
    {
      "type": "button",
      "label": "Import",
      "visibleOn": "${context.permissions && context.permissions.indexOf('hrm.employees.import') >= 0}",
      "actionType": "dialog",
      "dialog": "..."
    }
  ],
  "body": {
    "type": "crud",
    "api": "/hrm/employees",
    "columns": [
      {"name": "name", "label": "Name"},
      {"name": "email", "label": "Email"},
      {
        "type": "operation",
        "label": "Actions",
        "buttons": [
          {
            "label": "Edit",
            "visibleOn": "${context.permissions && context.permissions.indexOf('hrm.employees.edit') >= 0}",
            "actionType": "dialog",
            "dialog": "..."
          },
          {
            "label": "Delete",
            "visibleOn": "${context.permissions && context.permissions.indexOf('hrm.employees.delete') >= 0}",
            "actionType": "ajax",
            "api": "delete:/hrm/employees/${id}"
          }
        ]
      }
    ]
  }
}
```

### Helper Function for Permissions

```json
{
  "type": "tpl",
  "tpl": "${hasPermission(context.permissions, 'hrm.employees.edit') ? 'Can Edit' : 'Read Only'}"
}
```

Update `index.html` with helper:

```javascript
// Add helper function
amis.registerFilter('hasPermission', function(permissions, required) {
  if (!permissions || !required) return false;
  return permissions.indexOf(required) >= 0 || permissions.indexOf('*') >= 0;
});
```

---

## ABAC Implementation

### Attribute-Based Access Control

**More fine-grained than RBAC:**

```go
type ABACPolicy struct {
    ID         string                 `json:"id"`
    TenantID   string                 `json:"tenant_id"`
    Resource   string                 `json:"resource"`   // e.g., "hrm.employees"
    Action     string                 `json:"action"`     // e.g., "edit"
    Conditions map[string]any `json:"conditions"` // JSON conditions
}

// Example policies
[
  {
    "resource": "hrm.employees",
    "action": "edit",
    "conditions": {
      "user.department_id": "${resource.department_id}"  // Can only edit employees in same department
    }
  },
  {
    "resource": "hrm.salary",
    "action": "view",
    "conditions": {
      "user.role": "manager",
      "resource.level": {"$lte": "${user.level}"}  // Can only view salaries of lower levels
    }
  },
  {
    "resource": "sales.orders",
    "action": "approve",
    "conditions": {
      "resource.amount": {"$lte": 100000},  // Can only approve orders under 100,000
      "user.region": "${resource.region}"    // In same region
    }
  }
]
```

### ABAC Middleware

```go
func RequireABAC(resource, action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetString("tenant_id")
        userID := c.GetString("user_id")
        
        // Get user attributes
        user := getUserAttributes(userID, tenantID)
        
        // Get resource (from URL or body)
        resourceID := c.Param("id")
        resourceData := getResourceAttributes(resource, resourceID, tenantID)
        
        // Evaluate policy
        allowed := abacService.Evaluate(tenantID, resource, action, user, resourceData)
        
        if !allowed {
            c.JSON(403, gin.H{
                "status": 403,
                "msg":    "Access denied based on policy",
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// Usage
api.PUT("/hrm/employees/:id",
    RequireABAC("hrm.employees", "edit"),
    UpdateEmployee)
```

### Frontend: ABAC-Aware UI

```json
{
  "type": "crud",
  "api": "/hrm/employees",
  "columns": [
    {
      "type": "operation",
      "label": "Actions",
      "buttons": [
        {
          "label": "Edit",
          "actionType": "dialog",
          "visibleOn": "${this.department_id === context.user.department_id || context.user.role === 'admin'}",
          "dialog": "..."
        },
        {
          "label": "View Salary",
          "actionType": "drawer",
          "visibleOn": "${context.user.role === 'manager' && this.level <= context.user.level}",
          "drawer": "..."
        }
      ]
    }
  ]
}
```

---

## Localization (i18n)

### Backend: Translation Service

```go
type Translation struct {
    TenantID string            `json:"tenant_id"`
    Locale   string            `json:"locale"`
    Keys     map[string]string `json:"keys"`
}

func GetTranslations(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    locale := c.GetString("locale")
    
    var translation Translation
    db.Where("tenant_id = ? AND locale = ?", tenantID, locale).First(&translation)
    
    // Merge with default translations
    defaultTrans := getDefaultTranslations(locale)
    for k, v := range defaultTrans.Keys {
        if _, exists := translation.Keys[k]; !exists {
            translation.Keys[k] = v
        }
    }
    
    c.JSON(200, gin.H{
        "status": 0,
        "data":   translation.Keys,
    })
}
```

### Frontend: Load Translations

Update `index.html`:

```javascript
let translations = {};

function loadTranslations(locale) {
  return fetch(API_BASE + '/translations?locale=' + locale, {
    credentials: 'include'
  })
    .then(res => res.json())
    .then(response => {
      translations = response.data;
      
      // Register translation filter
      amis.registerFilter('t', function(key) {
        return translations[key] || key;
      });
    });
}

// Load translations before loading page
fetch(API_BASE + '/context')
  .then(res => res.json())
  .then(response => {
    userContext = response.data;
    return loadTranslations(userContext.locale);
  })
  .then(() => {
    navigate();
  });
```

### Use in Schemas

```json
{
  "type": "page",
  "title": "${'page.employees.title'|t}",
  "toolbar": [
    {
      "type": "button",
      "label": "${'button.add_employee'|t}",
      "actionType": "dialog",
      "dialog": {
        "title": "${'dialog.add_employee.title'|t}",
        "body": {
          "type": "form",
          "body": [
            {
              "type": "input-text",
              "name": "name",
              "label": "${'field.name'|t}",
              "placeholder": "${'field.name.placeholder'|t}",
              "required": true
            }
          ]
        }
      }
    }
  ]
}
```

### Translation Files

**English (en):**
```json
{
  "page.employees.title": "Employees",
  "button.add_employee": "Add Employee",
  "dialog.add_employee.title": "Add New Employee",
  "field.name": "Full Name",
  "field.name.placeholder": "Enter full name",
  "field.email": "Email Address",
  "field.department": "Department",
  "action.save": "Save",
  "action.cancel": "Cancel",
  "message.save_success": "Saved successfully",
  "message.delete_confirm": "Are you sure you want to delete this?"
}
```

**Swahili (sw):**
```json
{
  "page.employees.title": "Wafanyakazi",
  "button.add_employee": "Ongeza Mfanyakazi",
  "dialog.add_employee.title": "Ongeza Mfanyakazi Mpya",
  "field.name": "Jina Kamili",
  "field.name.placeholder": "Weka jina kamili",
  "field.email": "Barua Pepe",
  "field.department": "Idara",
  "action.save": "Hifadhi",
  "action.cancel": "Ghairi",
  "message.save_success": "Imehifadhiwa kikamilifu",
  "message.delete_confirm": "Una uhakika unataka kufuta?"
}
```

---

## Date/Time Formats

### Backend: Format Service

```go
func FormatDate(date time.Time, format, timezone string) string {
    loc, _ := time.LoadLocation(timezone)
    localTime := date.In(loc)
    
    // Convert format string (from config) to Go format
    goFormat := convertToGoFormat(format)
    
    return localTime.Format(goFormat)
}

func convertToGoFormat(format string) string {
    // Convert common formats to Go format
    replacements := map[string]string{
        "YYYY": "2006",
        "YY":   "06",
        "MM":   "01",
        "DD":   "02",
        "HH":   "15",
        "mm":   "04",
        "ss":   "05",
    }
    
    result := format
    for old, new := range replacements {
        result = strings.ReplaceAll(result, old, new)
    }
    
    return result
}
```

### Frontend: Date Filter

```javascript
// Register date format filter
amis.registerFilter('formatDate', function(value, format, timezone) {
  if (!value) return '';
  
  const date = new Date(value);
  const tz = timezone || userContext.timezone;
  
  // Use Intl or date-fns library
  const options = getDateOptions(format || userContext.date_format);
  
  return new Intl.DateTimeFormat(userContext.locale, options).format(date);
});

function getDateOptions(format) {
  // Convert format string to Intl options
  const options = {};
  
  if (format.includes('YYYY')) {
    options.year = 'numeric';
  }
  if (format.includes('MM')) {
    options.month = '2-digit';
  }
  if (format.includes('DD')) {
    options.day = '2-digit';
  }
  if (format.includes('HH')) {
    options.hour = '2-digit';
    options.hour12 = false;
  }
  
  return options;
}
```

### Use in Schemas

```json
{
  "name": "created_at",
  "label": "Created",
  "type": "tpl",
  "tpl": "${created_at|formatDate:context.date_format:context.timezone}"
}
```

---

## Currency & Number Formats

### Backend

```go
func FormatCurrency(amount float64, currency string, locale string) string {
    // Use currency formatting library
    return formatCurrency(amount, currency, locale)
}
```

### Frontend

```javascript
amis.registerFilter('currency', function(value, currency, locale) {
  if (typeof value !== 'number') return value;
  
  const cur = currency || userContext.currency;
  const loc = locale || userContext.locale;
  
  return new Intl.NumberFormat(loc, {
    style: 'currency',
    currency: cur
  }).format(value);
});

amis.registerFilter('number', function(value, decimals, locale) {
  if (typeof value !== 'number') return value;
  
  const loc = locale || userContext.locale;
  
  return new Intl.NumberFormat(loc, {
    minimumFractionDigits: decimals || 0,
    maximumFractionDigits: decimals || 2
  }).format(value);
});
```

### Use in Schemas

```json
{
  "name": "price",
  "label": "Price",
  "type": "tpl",
  "tpl": "${price|currency:context.currency:context.locale}"
}
```

---

## Tenant Branding

### Tenant Configuration

```go
type TenantBranding struct {
    TenantID      string `json:"tenant_id"`
    CompanyName   string `json:"company_name"`
    Logo          string `json:"logo"`
    PrimaryColor  string `json:"primary_color"`
    SecondaryColor string `json:"secondary_color"`
    FontFamily    string `json:"font_family"`
}
```

### Load Branding

```go
func GetTenantBranding(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    var branding TenantBranding
    db.Where("tenant_id = ?", tenantID).First(&branding)
    
    c.JSON(200, gin.H{
        "status": 0,
        "data":   branding,
    })
}
```

### Apply Branding in Frontend

```html
<style id="tenant-branding"></style>

<script>
fetch(API_BASE + '/branding')
  .then(res => res.json())
  .then(response => {
    const branding = response.data;
    
    // Apply CSS
    const css = `
      :root {
        --primary-color: ${branding.primary_color};
        --secondary-color: ${branding.secondary_color};
      }
      
      body {
        font-family: ${branding.font_family};
      }
      
      .btn-primary {
        background-color: ${branding.primary_color};
      }
      
      #sidebar .logo::before {
        content: url(${branding.logo});
      }
    `;
    
    document.getElementById('tenant-branding').textContent = css;
    document.title = branding.company_name + ' - AWO ERP';
  });
</script>
```

---

## Custom Fields

### Backend

```go
type CustomField struct {
    ID         string      `json:"id"`
    TenantID   string      `json:"tenant_id"`
    Module     string      `json:"module"`     // e.g., "hrm.employees"
    Name       string      `json:"name"`
    Label      string      `json:"label"`
    Type       string      `json:"type"`       // text, number, select, date, etc.
    Required   bool        `json:"required"`
    Options    any `json:"options"`    // For select/radio
    Validation any `json:"validation"`
}

func GetCustomFields(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    module := c.Query("module")
    
    var fields []CustomField
    db.Where("tenant_id = ? AND module = ?", tenantID, module).Find(&fields)
    
    c.JSON(200, gin.H{
        "status": 0,
        "data": gin.H{
            "fields": fields,
        },
    })
}
```

### Dynamic Form with Custom Fields

```json
{
  "type": "service",
  "api": "/custom-fields?module=hrm.employees",
  "body": {
    "type": "form",
    "api": "post:/hrm/employees",
    "body": [
      {
        "type": "input-text",
        "name": "name",
        "label": "Name",
        "required": true
      },
      {
        "type": "input-email",
        "name": "email",
        "label": "Email",
        "required": true
      },
      {
        "type": "combo",
        "name": "custom_fields",
        "label": "Additional Information",
        "items": "${fields}",
        "scaffold": {}
      }
    ]
  }
}
```

---

## Complete Multi-Tenant CRUD

**Complete example with all features:**

```json
{
  "type": "page",
  "title": "${'page.employees.title'|t}",
  
  "toolbar": [
    {
      "type": "button",
      "label": "${'button.add_employee'|t}",
      "level": "primary",
      "icon": "fa fa-plus",
      "actionType": "dialog",
      "visibleOn": "${hasPermission(context.permissions, 'hrm.employees.create') && hasFeature(context.feature_flags, 'hrm.employees')}",
      "dialog": {
        "title": "${'dialog.add_employee.title'|t}",
        "size": "lg",
        "body": {
          "type": "form",
          "api": "post:/hrm/employees",
          "body": [
            {
              "type": "input-text",
              "name": "name",
              "label": "${'field.name'|t}",
              "required": true
            },
            {
              "type": "input-email",
              "name": "email",
              "label": "${'field.email'|t}",
              "required": true
            },
            {
              "type": "select",
              "name": "department_id",
              "label": "${'field.department'|t}",
              "source": "/hrm/departments/options",
              "required": true,
              "creatable": true,
              "createBtnLabel": "${'button.add_department'|t}",
              "addApi": "post:/hrm/departments/quick-add",
              "addControls": [
                {
                  "type": "input-text",
                  "name": "name",
                  "label": "${'field.department_name'|t}",
                  "required": true
                }
              ]
            },
            {
              "type": "input-date",
              "name": "hire_date",
              "label": "${'field.hire_date'|t}",
              "format": "${context.date_format}",
              "required": true
            },
            {
              "type": "input-number",
              "name": "salary",
              "label": "${'field.salary'|t}",
              "prefix": "${context.currency}",
              "precision": 2,
              "visibleOn": "${hasPermission(context.permissions, 'hrm.salary.view')}"
            }
          ]
        }
      }
    }
  ],
  
  "body": {
    "type": "crud",
    "api": "/hrm/employees",
    "syncLocation": false,
    "perPage": "${context.config.ui.items_per_page || 20}",
    
    "filter": {
      "title": "${'filter.search'|t}",
      "body": [
        {
          "type": "input-text",
          "name": "keywords",
          "placeholder": "${'filter.keywords.placeholder'|t}",
          "clearable": true
        },
        {
          "type": "select",
          "name": "department_id",
          "label": "${'field.department'|t}",
          "source": "/hrm/departments/options",
          "clearable": true
        }
      ],
      "actions": [
        {
          "type": "reset",
          "label": "${'action.reset'|t}"
        },
        {
          "type": "submit",
          "label": "${'action.search'|t}",
          "level": "primary"
        }
      ]
    },
    
    "headerToolbar": [
      "bulkActions",
      "pagination",
      {
        "type": "export-excel",
        "label": "${'action.export'|t}",
        "api": "/hrm/employees?perPage=9999",
        "visibleOn": "${hasPermission(context.permissions, 'hrm.employees.export')}"
      }
    ],
    
    "columns": [
      {
        "name": "id",
        "label": "ID",
        "sortable": true,
        "width": 60
      },
      {
        "name": "name",
        "label": "${'field.name'|t}",
        "sortable": true
      },
      {
        "name": "email",
        "label": "${'field.email'|t}",
        "sortable": true
      },
      {
        "name": "department_name",
        "label": "${'field.department'|t}"
      },
      {
        "name": "hire_date",
        "label": "${'field.hire_date'|t}",
        "type": "tpl",
        "tpl": "${hire_date|formatDate:context.date_format:context.timezone}",
        "sortable": true
      },
      {
        "name": "salary",
        "label": "${'field.salary'|t}",
        "type": "tpl",
        "tpl": "${salary|currency:context.currency:context.locale}",
        "sortable": true,
        "visibleOn": "${hasPermission(context.permissions, 'hrm.salary.view')}"
      },
      {
        "name": "is_active",
        "label": "${'field.status'|t}",
        "type": "mapping",
        "map": {
          "1": "<span class='label label-success'>${'status.active'|t}</span>",
          "0": "<span class='label label-danger'>${'status.inactive'|t}</span>"
        }
      },
      {
        "type": "operation",
        "label": "${'column.actions'|t}",
        "buttons": [
          {
            "label": "${'action.edit'|t}",
            "type": "button",
            "level": "link",
            "icon": "fa fa-edit",
            "actionType": "drawer",
            "visibleOn": "${hasPermission(context.permissions, 'hrm.employees.edit') && (this.department_id === context.user.department_id || context.user.role === 'admin')}",
            "drawer": {
              "title": "${'dialog.edit_employee.title'|t}",
              "position": "right",
              "size": "md",
              "body": {
                "type": "form",
                "api": "put:/hrm/employees/${id}",
                "initApi": "/hrm/employees/${id}",
                "body": "..."
              }
            }
          },
          {
            "label": "${'action.delete'|t}",
            "type": "button",
            "level": "link",
            "className": "text-danger",
            "icon": "fa fa-trash",
            "actionType": "ajax",
            "api": "delete:/hrm/employees/${id}",
            "confirmText": "${'message.delete_confirm'|t}",
            "visibleOn": "${hasPermission(context.permissions, 'hrm.employees.delete')}"
          }
        ]
      }
    ]
  }
}
```

---

## Best Practices

### 1. Security

**Always validate on backend:**
```go
// Never trust frontend permissions/features
func UpdateEmployee(c *gin.Context) {
    // Re-verify permissions
    if !hasPermission(c, "hrm.employees.edit") {
        c.JSON(403, gin.H{"status": 403, "msg": "Access denied"})
        return
    }
    
    // Re-verify features
    if !hasFeature(c, "hrm.employees") {
        c.JSON(403, gin.H{"status": 403, "msg": "Feature not enabled"})
        return
    }
    
    // Process update
}
```

### 2. Performance

**Cache frequently accessed data:**
```go
// Cache tenant config, features, translations
var configCache = cache.New(5*time.Minute, 10*time.Minute)

func GetTenantConfig(tenantID string) *TenantConfig {
    if cached, found := configCache.Get(tenantID); found {
        return cached.(*TenantConfig)
    }
    
    config := loadConfigFromDB(tenantID)
    configCache.Set(tenantID, config, cache.DefaultExpiration)
    return config
}
```

### 3. Multi-Tenant Data Isolation

**Use PostgreSQL RLS:**
```sql
-- Enable RLS
ALTER TABLE employees ENABLE ROW LEVEL SECURITY;

-- Policy
CREATE POLICY tenant_isolation ON employees
    FOR ALL
    TO authenticated_user
    USING (tenant_id = current_setting('app.tenant_id')::uuid);
```

### 4. Audit Logging

**Track all tenant actions:**
```go
func AuditLog(c *gin.Context, action string, resource string, resourceID string, changes any) {
    log := AuditLog{
        TenantID:   c.GetString("tenant_id"),
        UserID:     c.GetString("user_id"),
        Action:     action,
        Resource:   resource,
        ResourceID: resourceID,
        Changes:    changes,
        IPAddress:  c.ClientIP(),
        Timestamp:  time.Now(),
    }
    
    db.Create(&log)
}
```

### 5. Feature Flag Rollout

**Gradual feature rollout:**
```go
type FeatureFlag struct {
    ID            string
    Name          string
    Enabled       bool
    RolloutPercent int      // 0-100
    WhitelistTenants []string
}

func IsFeatureEnabled(tenantID, featureID string) bool {
    flag := getFeatureFlag(featureID)
    
    // Check whitelist
    if contains(flag.WhitelistTenants, tenantID) {
        return true
    }
    
    // Check rollout percentage
    hash := hashTenantID(tenantID)
    if hash%100 < flag.RolloutPercent {
        return true
    }
    
    return flag.Enabled
}
```

### 6. Localization Best Practices

- Use translation keys, not hardcoded text
- Support RTL languages if needed
- Provide fallback to default language
- Allow tenant-specific translations

### 7. Testing Multi-Tenant Features

```go
func TestTenantIsolation(t *testing.T) {
    tenant1 := createTestTenant()
    tenant2 := createTestTenant()
    
    // Create employee for tenant1
    emp1 := createEmployee(tenant1.ID, "John")
    
    // Try to access from tenant2 - should fail
    ctx := setTenantContext(tenant2.ID)
    _, err := GetEmployee(ctx, emp1.ID)
    
    assert.Error(t, err)
    assert.Equal(t, "not found", err.Error())
}
```

---

## Summary

You've learned:

1. ✅ Advanced UI components (select with add-new, inline editing, etc.)
2. ✅ Multi-tenant architecture with feature flags
3. ✅ RBAC and ABAC implementation
4. ✅ Tenant-specific customization (localization, formats, branding)
5. ✅ Configuration-driven UIs
6. ✅ Identity service integration
7. ✅ Security best practices for multi-tenant systems

### Key Takeaways

- **Always validate on backend** - Never trust frontend
- **Cache aggressively** - Config, features, translations
- **Use RLS** - PostgreSQL row-level security for data isolation
- **Feature flags** - For gradual rollouts and A/B testing
- **Permissions everywhere** - In middleware, UI, and business logic
- **Audit everything** - Track all tenant actions
- **Test isolation** - Ensure tenants can't access each other's data

Your multi-tenant ERP system is now ready to serve multiple organizations with customized experiences! 
