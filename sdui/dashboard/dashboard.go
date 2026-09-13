// Package dashboard defines code-side dashboard definitions for the SDUI framework.
//
// In v1.0 dashboards are code-defined only — there is no tenant-configurable
// dashboard editor. Module authors declare DashboardDef values and register
// them via Register() from init().
//
// A DashboardDef describes a collection of panels (KPI cards, charts, tables,
// filter bars) laid out in a grid. The SDUI generator converts a DashboardDef
// into a widget.Node tree with the appropriate NodeKind values (NodeKPICard,
// NodeChartPanel, NodeTablePanel, NodeFilterBar).
//
// Example registration:
//
//	func init() {
//	    dashboard.Register(dashboard.DashboardDef{
//	        ID:    "finance.overview",
//	        Title: "Finance Overview",
//	        Panels: []dashboard.PanelDef{
//	            {Kind: dashboard.PanelKPI, ID: "total-revenue", Title: "Total Revenue", DataSource: "/api/v1/finance/kpi/revenue"},
//	            {Kind: dashboard.PanelChart, ID: "revenue-trend", Title: "Revenue Trend", ChartType: "line", DataSource: "/api/v1/finance/kpi/revenue-trend"},
//	        },
//	    })
//	}
package dashboard

import (
	"fmt"
	"sync"
)

// PanelKind is the type of a dashboard panel.
type PanelKind string

const (
	// PanelKPI is a key performance indicator metric card.
	PanelKPI PanelKind = "kpi"

	// PanelChart is a chart (bar, line, pie, area, scatter).
	PanelChart PanelKind = "chart"

	// PanelTable is a data table panel.
	PanelTable PanelKind = "table"

	// PanelFilter is a filter control bar (drives other panels on the same dashboard).
	PanelFilter PanelKind = "filter"
)

// KPIFormat specifies how a KPI value is displayed.
type KPIFormat string

const (
	KPIFormatNumber   KPIFormat = "number"
	KPIFormatCurrency KPIFormat = "currency"
	KPIFormatPercent  KPIFormat = "percent"
	KPIFormatDuration KPIFormat = "duration"
)

// ChartType specifies the chart variant.
type ChartType string

const (
	ChartBar     ChartType = "bar"
	ChartLine    ChartType = "line"
	ChartPie     ChartType = "pie"
	ChartArea    ChartType = "area"
	ChartScatter ChartType = "scatter"
)

// PanelDef describes a single panel on a dashboard.
type PanelDef struct {
	// ID is the stable panel identifier. Must be unique within the DashboardDef.
	ID string

	// Kind is the panel type (KPI, Chart, Table, Filter).
	Kind PanelKind

	// Title is the panel display title.
	Title string

	// DataSource is the API endpoint for this panel's data.
	// Required for KPI, Chart, and Table panels.
	// Optional for Filter panels (which drive other panels).
	DataSource string

	// ChartType specifies the chart variant. Only relevant for PanelChart.
	ChartType ChartType

	// KPIFormat specifies the KPI display format. Only relevant for PanelKPI.
	KPIFormat KPIFormat

	// ValueField is the data field name for the KPI value.
	ValueField string

	// ColSpan is the number of grid columns this panel occupies (1–12).
	// Zero means the renderer chooses the default.
	ColSpan int

	// Permissions lists permission identifiers required to view this panel.
	// Empty means the panel is visible to all viewers who can access the dashboard.
	Permissions []string
}

// DashboardDef defines a complete dashboard page.
type DashboardDef struct {
	// ID is the stable dashboard identifier (e.g., "finance.overview").
	// Must be unique across all registered dashboards.
	ID string

	// Title is the dashboard display title.
	Title string

	// Module is the module that owns this dashboard (e.g., "finance", "crm").
	Module string

	// Permissions lists permission identifiers required to access this dashboard.
	// Empty means any authenticated viewer can access it.
	Permissions []string

	// Panels are the dashboard panels in display order.
	Panels []PanelDef
}

// Registry holds registered DashboardDefs.
type Registry struct {
	mu   sync.RWMutex
	defs map[string]DashboardDef
}

// global is the production dashboard registry singleton.
var global = &Registry{defs: make(map[string]DashboardDef)}

// Register adds a DashboardDef to the global registry.
// Panics if a dashboard with the same ID is already registered.
// Call from init() only.
func Register(def DashboardDef) {
	if err := global.Register(def); err != nil {
		panic(fmt.Sprintf("dashboard.Register: %v", err))
	}
}

// Lookup returns the DashboardDef for the given ID from the global registry.
func Lookup(id string) (DashboardDef, bool) { return global.Lookup(id) }

// All returns all registered DashboardDefs from the global registry.
func All() []DashboardDef { return global.All() }

// NewRegistry returns an isolated registry for testing.
func NewRegistry() *Registry { return &Registry{defs: make(map[string]DashboardDef)} }

// ── Instance methods ──────────────────────────────────────────────────────────

// Register adds a DashboardDef to the registry.
func (r *Registry) Register(def DashboardDef) error {
	if def.ID == "" {
		return fmt.Errorf("DashboardDef.ID must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.defs[def.ID]; exists {
		return fmt.Errorf("dashboard %q is already registered (duplicate ID)", def.ID)
	}
	// Validate panel IDs for uniqueness within the dashboard.
	seen := make(map[string]bool)
	for _, panel := range def.Panels {
		if panel.ID == "" {
			return fmt.Errorf("dashboard %q: PanelDef.ID must not be empty", def.ID)
		}
		if seen[panel.ID] {
			return fmt.Errorf("dashboard %q: duplicate panel ID %q", def.ID, panel.ID)
		}
		seen[panel.ID] = true
	}
	r.defs[def.ID] = def
	return nil
}

// Lookup returns the DashboardDef for the given ID.
func (r *Registry) Lookup(id string) (DashboardDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.defs[id]
	return def, ok
}

// All returns all registered DashboardDefs as a slice.
func (r *Registry) All() []DashboardDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DashboardDef, 0, len(r.defs))
	for _, def := range r.defs {
		out = append(out, def)
	}
	return out
}
