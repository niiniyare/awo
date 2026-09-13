// Package amis implements the AMIS renderer for the SDUI framework.
//
// The AMIS renderer translates a validated widget.Node tree into an AMIS JSON
// schema (map[string]any) ready for serialisation and delivery to the browser.
//
// ADR-006: The amis renderer is one possible renderer for the WidgetTree IR.
// It MUST NOT accept raw field definitions or entity definitions — its sole
// input is always widget.Node values produced by the generator.
//
// This package is the only place in the SDUI framework that produces
// AMIS-specific output. No AMIS-specific constructs may appear in the
// generator, widget, or any other package.
//
// Normative requirements:
//   - Render returns an error for any unknown NodeKind.
//   - Node.Hidden == true causes the node to be omitted from output entirely.
//   - Node.Props is merged last and overrides all computed defaults.
//   - ExpressionRef values are serialised via expression.AMISSerializer.
//   - Renderer is safe for concurrent use after construction.
//   - Output is deterministic for the same input tree and RendererContext.
package amis

import (
	"fmt"
	"strings"

	"awo.so/awo/sdui/expression"
	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/widget"
)

const (
	// RendererID is the stable identifier for the AMIS renderer.
	RendererID = "amis"

	// RendererVersion is the version token included in cache keys.
	// Increment when the renderer output format changes.
	RendererVersion = "1.1.0"
)

// DefaultRenderer is the production AMIS renderer.
// It handles all NodeKind constants defined in awo/sdui/widget.
// Construct via New(). Safe for concurrent use.
type DefaultRenderer struct {
	exprSerializer expression.AMISSerializer
}

// New returns a DefaultRenderer ready for use.
func New() *DefaultRenderer {
	return &DefaultRenderer{}
}

// ID returns the renderer identifier.
func (r *DefaultRenderer) ID() string { return RendererID }

// Version returns the renderer version token for cache key construction.
func (r *DefaultRenderer) Version() string { return RendererVersion }

// Render translates root and all its descendants into an AMIS page schema.
// Unknown NodeKind values cause Render to return an error — they are never
// silently dropped or rendered as empty divs.
func (r *DefaultRenderer) Render(root *widget.Node, ctx renderer.RendererContext) (renderer.RenderedOutput, error) {
	if root == nil {
		return renderer.RenderedOutput{}, fmt.Errorf("amis.Render: root node must not be nil")
	}
	schema, err := r.renderNode(root, ctx)
	if err != nil {
		return renderer.RenderedOutput{}, err
	}
	return renderer.NewAMISOutput(schema), nil
}

// renderNode converts a single Node and its subtree to an amis schema object.
func (r *DefaultRenderer) renderNode(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	if n == nil {
		return nil, fmt.Errorf("amis.renderNode: nil node in tree")
	}

	// Hidden nodes are entirely omitted — not rendered with hidden:true.
	// (See WIDGET_TREE_SPEC.md §7 — permission-gated elements must be absent.)
	if n.Hidden {
		return nil, nil
	}

	var out map[string]any
	var err error

	switch n.Kind {
	case widget.NodePage:
		out, err = r.renderPage(n, ctx)
	case widget.NodeForm:
		out, err = r.renderForm(n, ctx)
	case widget.NodeList:
		out, err = r.renderList(n, ctx)
	case widget.NodeSection:
		out, err = r.renderSection(n, ctx)
	case widget.NodeTabPane:
		out, err = r.renderTabPane(n, ctx)
	case widget.NodeTabs:
		out, err = r.renderTabs(n, ctx)
	case widget.NodeTable:
		out, err = r.renderTable(n, ctx)
	case widget.NodeGrid:
		out, err = r.renderGrid(n, ctx)
	case widget.NodeDialog:
		out, err = r.renderDialog(n, ctx)
	case widget.NodeButton:
		out, err = r.renderButton(n, ctx)
	case widget.NodeText, widget.NodeField:
		out, err = r.renderText(n)
	case widget.NodeTextArea:
		out, err = r.renderTextArea(n)
	case widget.NodeRichText:
		out, err = r.renderRichText(n)
	case widget.NodeNumber:
		out, err = r.renderNumber(n, ctx)
	case widget.NodeMoney:
		out, err = r.renderMoney(n, ctx)
	case widget.NodeSelect:
		out, err = r.renderSelect(n)
	case widget.NodeMultiSelect:
		out, err = r.renderMultiSelect(n)
	case widget.NodeLookup:
		out, err = r.renderLookup(n)
	case widget.NodeTreeSelect:
		out, err = r.renderTreeSelect(n)
	case widget.NodeDate:
		out, err = r.renderDate(n, ctx)
	case widget.NodeDateTime:
		out, err = r.renderDateTime(n, ctx)
	case widget.NodeDuration:
		out, err = r.renderDuration(n)
	case widget.NodeSwitch:
		out, err = r.renderSwitch(n)
	case widget.NodeEditor:
		out, err = r.renderEditor(n)
	case widget.NodeColor:
		out, err = r.renderColor(n)
	case widget.NodeSignature:
		out, err = r.renderSignature(n)
	case widget.NodeFileUpload:
		out, err = r.renderFileUpload(n)
	case widget.NodeStaticText:
		out, err = r.renderStaticText(n)
	case widget.NodeBadge:
		out, err = r.renderBadge(n)
	case widget.NodeSummaryCard:
		out, err = r.renderSummaryCard(n, ctx)
	case widget.NodeWorkflowPanel:
		out, err = r.renderWorkflowPanel(n)
	case widget.NodeAttachments:
		out, err = r.renderAttachments(n)
	case widget.NodeActivity:
		out, err = r.renderActivity(n)
	case widget.NodeRelatedList:
		out, err = r.renderRelatedList(n, ctx)
	case widget.NodeKPICard:
		out, err = r.renderKPICard(n)
	case widget.NodeChartPanel:
		out, err = r.renderChartPanel(n)
	case widget.NodeTablePanel:
		out, err = r.renderTablePanel(n, ctx)
	case widget.NodeFilterBar:
		out, err = r.renderFilterBar(n, ctx)
	default:
		return nil, fmt.Errorf("amis.renderNode: unknown NodeKind %q — all NodeKinds must be handled", n.Kind)
	}

	if err != nil {
		return nil, err
	}

	// Apply expressions from portable ExpressionRef values.
	if err := r.applyExpressions(n, out); err != nil {
		return nil, err
	}

	// Merge Node.Props last — they override all computed defaults.
	for k, v := range n.Props {
		out[k] = v
	}

	// Set stable ID if present.
	if n.ID != "" {
		out["id"] = n.ID
	}

	return out, nil
}

// applyExpressions serialises portable ExpressionRef values to AMIS JS strings.
func (r *DefaultRenderer) applyExpressions(n *widget.Node, out map[string]any) error {
	if s, err := r.serializeRef(n.VisibleOn, n.ID, "VisibleOn"); err != nil {
		return err
	} else if s != "" {
		out["visibleOn"] = s
	}
	if s, err := r.serializeRef(n.HiddenOn, n.ID, "HiddenOn"); err != nil {
		return err
	} else if s != "" {
		out["hiddenOn"] = s
	}
	if s, err := r.serializeRef(n.DisabledOn, n.ID, "DisabledOn"); err != nil {
		return err
	} else if s != "" {
		out["disabledOn"] = s
	}
	if s, err := r.serializeRef(n.RequiredOn, n.ID, "RequiredOn"); err != nil {
		return err
	} else if s != "" {
		out["requiredOn"] = s
	}
	return nil
}

// serializeRef converts an ExpressionRef to an AMIS JS string.
// Supports two Expr types:
//   - string: raw JS expression — passed through as-is (legacy/compat path)
//   - expression.ExpressionNode: serialised via AMISSerializer
func (r *DefaultRenderer) serializeRef(ref *widget.ExpressionRef, nodeID, field string) (string, error) {
	if ref == nil || ref.Expr == nil {
		return "", nil
	}
	switch v := ref.Expr.(type) {
	case string:
		return v, nil
	case expression.ExpressionNode:
		s, err := r.exprSerializer.Serialize(v)
		if err != nil {
			return "", fmt.Errorf("amis: %s serialisation for node %q: %w", field, nodeID, err)
		}
		return s, nil
	default:
		return "", fmt.Errorf("amis: %s.Expr on node %q is unsupported type %T", field, nodeID, ref.Expr)
	}
}

// renderChildren converts a slice of Nodes into amis schema objects.
// Nil results (from Hidden nodes) are omitted.
func (r *DefaultRenderer) renderChildren(nodes []*widget.Node, ctx renderer.RendererContext) ([]any, error) {
	var out []any
	for _, child := range nodes {
		m, err := r.renderNode(child, ctx)
		if err != nil {
			return nil, err
		}
		if m != nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// renderActions converts ActionNode values to amis button schema objects.
func (r *DefaultRenderer) renderActions(actions []*widget.ActionNode) ([]any, error) {
	out := make([]any, 0, len(actions))
	for _, a := range actions {
		if a == nil {
			continue
		}
		btn := map[string]any{
			"type":       "button",
			"label":      sanitizeText(a.Label),
			"actionType": a.ActionType,
			"level":      a.Level,
		}
		if a.ID != "" {
			btn["id"] = a.ID
		}
		if a.Href != "" {
			btn["link"] = a.Href
		}
		if a.API != "" {
			btn["api"] = a.API
		}
		if a.ConfirmText != "" {
			btn["confirmText"] = sanitizeText(a.ConfirmText)
		}
		if a.Icon != "" {
			btn["icon"] = "fa fa-" + a.Icon
		}
		if a.DialogTarget != "" {
			btn["dialog"] = map[string]any{"target": a.DialogTarget}
		}
		// Serialize action expressions.
		if a.VisibleOn != nil && a.VisibleOn.Expr != nil {
			if expr, ok := a.VisibleOn.Expr.(expression.ExpressionNode); ok {
				if s, err := r.exprSerializer.Serialize(expr); err == nil {
					btn["visibleOn"] = s
				}
			}
		}
		if a.DisabledOn != nil && a.DisabledOn.Expr != nil {
			if expr, ok := a.DisabledOn.Expr.(expression.ExpressionNode); ok {
				if s, err := r.exprSerializer.Serialize(expr); err == nil {
					btn["disabledOn"] = s
				}
			}
		}
		out = append(out, btn)
	}
	return out, nil
}

// applyCommon sets name, label, required, disabled, description, placeholder,
// and validation rules on a field schema from the typed Node fields.
func applyCommon(n *widget.Node, out map[string]any) {
	if n.Name != "" {
		out["name"] = n.Name
	}
	if n.Label != "" {
		out["label"] = sanitizeText(n.Label)
	}
	if n.RequiredOn == nil {
		if n.Required {
			out["required"] = true
		}
	}
	if n.DisabledOn == nil {
		if n.ReadOnly {
			out["disabled"] = true
		}
	}
	if n.Description != "" {
		out["description"] = sanitizeText(n.Description)
	}
	if n.Placeholder != "" {
		out["placeholder"] = sanitizeText(n.Placeholder)
	}
	if n.MaxLength > 0 {
		out["maxLength"] = n.MaxLength
	}
	if n.MinValue != nil {
		out["min"] = *n.MinValue
	}
	if n.MaxValue != nil {
		out["max"] = *n.MaxValue
	}
	if n.Icon != "" {
		out["prefix"] = "<i class=\"fa fa-" + n.Icon + "\"></i>"
	}
	if n.Layout != nil && n.Layout.Width != "" {
		out["size"] = n.Layout.Width
	}
	if len(n.ClearOn) > 0 {
		out["clearValueNotMatch"] = true
		out["clearOn"] = n.ClearOn
	}
	// Validation rules.
	if len(n.Validation) > 0 {
		validations := map[string]any{}
		messages := map[string]any{}
		for _, rule := range n.Validation {
			validations[rule.Type] = rule.Value
			if rule.Message != "" {
				messages[rule.Type] = rule.Message
			}
		}
		out["validations"] = validations
		if len(messages) > 0 {
			out["validationErrors"] = messages
		}
	}
}

// labelReplacer strips angle-bracket characters from label strings.
// See sanitizeText for rationale.
var labelReplacer = strings.NewReplacer("<", "", ">", "")

// sanitizeText normalises label, description, and confirmText strings before
// inclusion in the AMIS schema JSON.
//
// # Design
//
// AMIS renders label/description/confirmText as React text nodes — NOT via
// innerHTML — so HTML encoding is NOT correct here:
//   - html.EscapeString("R&D") = "R&amp;D", which React displays as "R&amp;D"
//     literally (the text node is not interpreted as HTML).
//   - encoding/json handles all JSON string encoding; no additional encoding needed.
//
// # Sanitization
//
// Angle brackets (<, >) are stripped as a defense-in-depth measure. Entity
// metadata labels are Go compile-time constants and will never legitimately
// contain angle brackets. Stripping prevents HTML injection if any AMIS
// component ever renders a label via innerHTML (e.g., a custom renderer,
// tooltip, or future AMIS version change).
//
// The ampersand (&) is intentionally NOT encoded. Encoding it would produce
// "&amp;" visible literally in React text nodes.
func sanitizeText(s string) string {
	return labelReplacer.Replace(strings.TrimSpace(s))
}

// ── Structural renderers ──────────────────────────────────────────────────────

func (r *DefaultRenderer) renderPage(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	body, err := r.renderChildren(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"type": "page"}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	if len(body) > 0 {
		out["body"] = body
	}
	if acts, err := r.renderActions(n.Actions); err != nil {
		return nil, err
	} else if len(acts) > 0 {
		out["toolbar"] = acts
	}
	if ctx.RTL {
		out["dir"] = "rtl"
	}
	applyTheme(out, ctx.Theme)
	return out, nil
}

func (r *DefaultRenderer) renderForm(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	body, err := r.renderChildren(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type": "form",
		"body": body,
	}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["api"] = buildAPI(n.DataSource)
	}
	if n.DataSource != nil && n.DataSource.ReadURL != "" {
		out["initApi"] = "GET:" + n.DataSource.ReadURL
	}
	if acts, err := r.renderActions(n.Actions); err != nil {
		return nil, err
	} else if len(acts) > 0 {
		out["actions"] = acts
	}
	return out, nil
}

func (r *DefaultRenderer) renderList(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	columns, err := r.renderListColumns(n.Children, ctx)
	if err != nil {
		return nil, err
	}

	// Partition actions by Scope.
	// The generator is authoritative: it gates row/bulk ops on viewer permissions.
	// The renderer only routes actions to their correct AMIS slot.
	var toolbarActions, rowActions, bulkActions []*widget.ActionNode
	for _, a := range n.Actions {
		if a == nil {
			continue
		}
		switch a.Scope {
		case "row":
			rowActions = append(rowActions, a)
		case "bulk":
			bulkActions = append(bulkActions, a)
		default: // "toolbar" or empty — toolbar is the default slot
			toolbarActions = append(toolbarActions, a)
		}
	}

	// Row operations column — rendered only when the generator provides row actions.
	// This makes delete/edit buttons permission-aware: they appear only when the
	// generator decided the viewer has the required permission.
	if len(rowActions) > 0 {
		rowBtns, err := r.renderRowActions(rowActions)
		if err != nil {
			return nil, err
		}
		columns = append(columns, map[string]any{
			"type":    "operation",
			"label":   "Actions",
			"width":   rowOperationWidth(rowActions),
			"buttons": rowBtns,
		})
	}

	// Render toolbar action buttons (create, custom list-scope actions).
	acts, err := r.renderActions(toolbarActions)
	if err != nil {
		return nil, err
	}

	// Header toolbar: action buttons first, then reload.
	// Pagination lives in footerToolbar (standard crud pattern).
	// String shortcuts "reload" / "statistics" / "pagination" are supported by
	// the crud component. crud2 does NOT honour these shortcuts — it renders
	// them as literal text, which is why we use crud here.
	headerToolbar := make([]any, 0, len(acts)+1)
	headerToolbar = append(headerToolbar, acts...)
	headerToolbar = append(headerToolbar, "reload")

	// Bulk actions — rendered only when generator provided bulk ops.
	// renderedBulk must be a non-nil slice: "bulkActions": null serialises
	// differently from "bulkActions": [] and AMIS may show a placeholder for null.
	renderedBulk := []any{}
	if len(bulkActions) > 0 {
		renderedBulk, err = r.renderActions(bulkActions)
		if err != nil {
			return nil, err
		}
	}

	out := map[string]any{
		// Use "crud" not "crud2": crud honours string toolbar shortcuts
		// ("reload", "statistics", "pagination"). crud2 ignores them and renders
		// the string value literally — producing "reloadpagination" in the UI.
		"type":             "crud",
		"columns":          columns,
		"syncLocation":     false,
		"perPage":          20,
		"perPageAvailable": []int{10, 20, 50, 100},
		"headerToolbar":    headerToolbar,
		"footerToolbar":    []any{"statistics", "pagination"},
		// bulkActions must always be present. Empty slice suppresses the AMIS
		// bulk-action placeholder text. Non-empty enables row checkboxes.
		"bulkActions": renderedBulk,
		// itemsKey must match what adaptResponse() produces in the frontend fetcher.
		// AMIS crud defaults to "rows"; our backend wraps list data as { items: [...] }.
		// Setting this explicitly prevents breakage on AMIS version drift.
		"itemsKey": "items",
		// English empty-state text wired directly; does not depend on locale bundle.
		"placeholder": "No records found.",
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["api"] = buildAPI(n.DataSource)
	}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	// Wire filter bar as the crud search form.
	if n.FilterBar != nil {
		filter, err := r.renderFilterBar(n.FilterBar, ctx)
		if err != nil {
			return nil, err
		}
		if filter != nil {
			out["filter"] = filter
		}
	}
	return out, nil
}

// renderRowActions converts row-scoped ActionNode values to AMIS button maps
// sized for the per-row operation column.
func (r *DefaultRenderer) renderRowActions(actions []*widget.ActionNode) ([]any, error) {
	out := make([]any, 0, len(actions))
	for _, a := range actions {
		if a == nil {
			continue
		}
		btn := map[string]any{
			"type":  "button",
			"label": sanitizeText(a.Label),
			"level": a.Level,
			"size":  "sm",
		}
		if a.ActionType != "" {
			btn["actionType"] = a.ActionType
		}
		if a.Href != "" {
			btn["link"] = a.Href
			btn["actionType"] = "link"
		}
		if a.API != "" {
			btn["api"] = a.API
		}
		if a.ConfirmText != "" {
			btn["confirmText"] = sanitizeText(a.ConfirmText)
		}
		if a.Icon != "" {
			btn["icon"] = "fa fa-" + a.Icon
		}
		// Danger-level row actions get a CSS class for colour contrast.
		if a.Level == "danger" || a.ID == "delete" {
			btn["className"] = "text-danger"
		}
		out = append(out, btn)
	}
	return out, nil
}

// rowOperationWidth returns a column width that fits the given row actions.
// Approximate widths: 60px per action, minimum 100px.
func rowOperationWidth(actions []*widget.ActionNode) int {
	w := len(actions) * 65
	if w < 100 {
		w = 100
	}
	return w
}

func (r *DefaultRenderer) renderSection(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	body, err := r.renderChildren(n.Children, ctx)
	if err != nil {
		return nil, err
	}

	if n.Label != "" || n.Collapsible {
		out := map[string]any{
			"type": "fieldSet",
			"body": body,
		}
		if n.Label != "" {
			out["title"] = sanitizeText(n.Label)
		}
		if n.Collapsible {
			out["collapsable"] = true // AMIS uses "collapsable" (not "collapsible")
			out["collapsed"] = n.Collapsed
		}
		return out, nil
	}

	return map[string]any{
		"type": "group",
		"body": body,
	}, nil
}

func (r *DefaultRenderer) renderTabPane(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	body, err := r.renderChildren(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"title": sanitizeText(n.Label),
		"body":  body,
	}
	return out, nil
}

func (r *DefaultRenderer) renderTabs(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	var tabs []any
	for _, child := range n.Children {
		if child == nil || child.Hidden {
			continue
		}
		// Children must be NodeTabPane (validated before rendering).
		tab, err := r.renderNode(child, ctx)
		if err != nil {
			return nil, err
		}
		if tab != nil {
			tabs = append(tabs, tab)
		}
	}
	return map[string]any{
		"type": "tabs",
		"tabs": tabs,
	}, nil
}

func (r *DefaultRenderer) renderTable(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	cols, err := r.renderListColumns(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type":    "table",
		"columns": cols,
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = n.DataSource.URL
	}
	if n.Label != "" {
		out["label"] = sanitizeText(n.Label)
	}
	return out, nil
}

func (r *DefaultRenderer) renderGrid(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	cols, err := r.renderListColumns(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type":    "input-table",
		"columns": cols,
	}
	if n.Name != "" {
		out["name"] = n.Name
	}
	if n.Label != "" {
		out["label"] = sanitizeText(n.Label)
	}
	if n.GridEditable {
		out["addable"] = true
		out["editable"] = true
		out["removable"] = true
	}
	if n.GridMinRows > 0 {
		out["minLength"] = n.GridMinRows
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = n.DataSource.URL
	}
	return out, nil
}

func (r *DefaultRenderer) renderDialog(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	body, err := r.renderChildren(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type": "dialog",
		"body": body,
	}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	return out, nil
}

// ── Field renderers ───────────────────────────────────────────────────────────

func (r *DefaultRenderer) renderButton(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	out := map[string]any{
		"type":  "button",
		"label": sanitizeText(n.Label),
	}
	applyCommon(n, out) // applies disabled/disabledOn, expressions
	return out, nil
}

func (r *DefaultRenderer) renderText(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "input-text"}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderTextArea(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "textarea"}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderRichText(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "rich-text"}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderNumber(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	out := map[string]any{"type": "input-number"}
	applyCommon(n, out)
	if ctx.DecimalSeparator != "" && ctx.DecimalSeparator != "." {
		out["decimalSeparator"] = ctx.DecimalSeparator
	}
	if ctx.ThousandSeparator != "" {
		out["thousandSeparator"] = ctx.ThousandSeparator
	}
	return out, nil
}

func (r *DefaultRenderer) renderMoney(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	out := map[string]any{"type": "input-number"}
	applyCommon(n, out)
	out["precision"] = 2
	if ctx.DecimalSeparator != "" && ctx.DecimalSeparator != "." {
		out["decimalSeparator"] = ctx.DecimalSeparator
	}
	if ctx.ThousandSeparator != "" {
		out["thousandSeparator"] = ctx.ThousandSeparator
	}
	if n.CurrencyField != "" {
		// AMIS does not have a native money+currency combined widget.
		// Emit as a group with the number input and a companion select.
		// The generator is responsible for emitting NodeMoney; the renderer
		// chooses the best AMIS representation.
		out["step"] = 0.01
		out["suffix"] = "${" + n.CurrencyField + "}"
	}
	return out, nil
}

func (r *DefaultRenderer) renderSelect(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "select"}
	applyCommon(n, out)
	if n.DataSource != nil && n.DataSource.URL != "" {
		src := buildSourceMap(n.DataSource)
		out["source"] = src
		applyLabelValueFields(n.DataSource, out)
	} else if len(n.Options) > 0 {
		opts := make([]map[string]any, len(n.Options))
		for i, o := range n.Options {
			opts[i] = map[string]any{"label": sanitizeText(o.Label), "value": o.Value}
		}
		out["options"] = opts
	}
	return out, nil
}

func (r *DefaultRenderer) renderMultiSelect(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type":     "select",
		"multiple": true,
	}
	applyCommon(n, out)
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = buildSourceMap(n.DataSource)
		applyLabelValueFields(n.DataSource, out)
	} else if len(n.Options) > 0 {
		opts := make([]map[string]any, len(n.Options))
		for i, o := range n.Options {
			opts[i] = map[string]any{"label": sanitizeText(o.Label), "value": o.Value}
		}
		out["options"] = opts
	}
	return out, nil
}

func (r *DefaultRenderer) renderLookup(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type":         "select",
		"searchable":   true,
		"autoComplete": true,
	}
	applyCommon(n, out)
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = buildSourceMap(n.DataSource)
		applyLabelValueFields(n.DataSource, out)
	}
	return out, nil
}

func (r *DefaultRenderer) renderTreeSelect(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "tree-select"}
	applyCommon(n, out)
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = buildSourceMap(n.DataSource)
		applyLabelValueFields(n.DataSource, out)
	}
	if n.TreeParentField != "" {
		out["parentField"] = n.TreeParentField
	}
	return out, nil
}

func (r *DefaultRenderer) renderDate(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	format := "YYYY-MM-DD"
	if ctx.DateFormat != "" {
		format = amisDateFormat(ctx.DateFormat)
	}
	out := map[string]any{
		"type":   "input-date",
		"format": format,
	}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderDateTime(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	format := "YYYY-MM-DDTHH:mm:ssZ"
	if ctx.DateTimeFormat != "" {
		format = amisDateFormat(ctx.DateTimeFormat)
	}
	out := map[string]any{
		"type":   "input-datetime",
		"format": format,
	}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderDuration(n *widget.Node) (map[string]any, error) {
	// AMIS does not have a native duration picker. Render as a masked input-text.
	out := map[string]any{
		"type":        "input-text",
		"placeholder": "HH:MM:SS",
	}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderSwitch(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "switch"}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderEditor(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type":     "json-editor",
		"language": "json",
	}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderColor(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "input-color"}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderSignature(n *widget.Node) (map[string]any, error) {
	// AMIS does not have a native signature widget. Render as a file upload.
	// A custom AMIS component registration is required for production use.
	out := map[string]any{"type": "input-file"}
	applyCommon(n, out)
	return out, nil
}

func (r *DefaultRenderer) renderFileUpload(n *widget.Node) (map[string]any, error) {
	out := map[string]any{"type": "input-file"}
	applyCommon(n, out)
	if len(n.AcceptedTypes) > 0 {
		out["accept"] = strings.Join(n.AcceptedTypes, ",")
	}
	if n.MaxFileSize > 0 {
		out["maxSize"] = n.MaxFileSize
	}
	return out, nil
}

func (r *DefaultRenderer) renderStaticText(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type":     "tpl",
		"tpl":      sanitizeText(n.StaticContent),
		"wrapName": false,
	}
	if n.Label != "" {
		out["label"] = sanitizeText(n.Label)
	}
	return out, nil
}

func (r *DefaultRenderer) renderBadge(n *widget.Node) (map[string]any, error) {
	colorMap := map[string]string{
		"success": "success",
		"warning": "warning",
		"danger":  "danger",
		"info":    "info",
		"default": "default",
	}
	color := "default"
	if c, ok := colorMap[n.BadgeColor]; ok {
		color = c
	}
	out := map[string]any{
		"type": "mapping",
		"name": n.Name,
		"map": map[string]any{
			"*": "<span class=\"label label-" + color + "\">${value}</span>",
		},
	}
	if n.Label != "" {
		out["label"] = sanitizeText(n.Label)
	}
	return out, nil
}

func (r *DefaultRenderer) renderSummaryCard(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	body, err := r.renderChildren(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type": "panel",
		"body": body,
	}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	if acts, err := r.renderActions(n.Actions); err != nil {
		return nil, err
	} else if len(acts) > 0 {
		out["actions"] = acts
	}
	return out, nil
}

func (r *DefaultRenderer) renderWorkflowPanel(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type": "steps",
		"name": n.Name,
	}
	if n.Label != "" {
		out["label"] = sanitizeText(n.Label)
	}
	if n.WorkflowID != "" {
		out["source"] = map[string]any{
			"url":    "${workflowStateURL}",
			"method": "GET",
		}
	}
	return out, nil
}

func (r *DefaultRenderer) renderAttachments(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type":     "input-file",
		"name":     n.Name,
		"multiple": true,
	}
	if n.Label != "" {
		out["label"] = sanitizeText(n.Label)
	}
	return out, nil
}

func (r *DefaultRenderer) renderActivity(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type": "timeline",
		"name": n.Name,
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = n.DataSource.URL
	}
	return out, nil
}

func (r *DefaultRenderer) renderRelatedList(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	cols, err := r.renderListColumns(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type":          "crud",
		"columns":       cols,
		"footerToolbar": []any{"statistics", "pagination"},
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["api"] = buildAPI(n.DataSource)
	}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	if acts, err := r.renderActions(n.Actions); err != nil {
		return nil, err
	} else if len(acts) > 0 {
		out["toolbar"] = acts
	}
	return out, nil
}

func (r *DefaultRenderer) renderKPICard(n *widget.Node) (map[string]any, error) {
	out := map[string]any{
		"type": "card",
		"body": map[string]any{
			"type": "tpl",
			"tpl":  "<div class=\"kpi-card\"><div class=\"kpi-label\">" + sanitizeText(n.Label) + "</div><div class=\"kpi-value\">${" + n.Name + "}</div></div>",
		},
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = n.DataSource.URL
	}
	return out, nil
}

func (r *DefaultRenderer) renderChartPanel(n *widget.Node) (map[string]any, error) {
	chartType := n.ChartType
	if chartType == "" {
		chartType = "bar"
	}
	out := map[string]any{
		"type":   "chart",
		"config": map[string]any{"type": chartType},
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["api"] = buildAPI(n.DataSource)
	}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	return out, nil
}

func (r *DefaultRenderer) renderTablePanel(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	cols, err := r.renderListColumns(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type":    "table",
		"columns": cols,
	}
	if n.DataSource != nil && n.DataSource.URL != "" {
		out["source"] = n.DataSource.URL
	}
	if n.Label != "" {
		out["title"] = sanitizeText(n.Label)
	}
	return out, nil
}

func (r *DefaultRenderer) renderFilterBar(n *widget.Node, ctx renderer.RendererContext) (map[string]any, error) {
	body, err := r.renderChildren(n.Children, ctx)
	if err != nil {
		return nil, err
	}
	// submitText/resetText are explicit English strings so the filter bar labels
	// are correct regardless of whether the locale bundle loaded successfully.
	// wrapWithPanel:false prevents AMIS from wrapping the filter in a card/panel
	// when it is embedded as crud.filter — crud provides its own container.
	return map[string]any{
		"type":           "form",
		"mode":           "horizontal",
		"body":           body,
		"submitText":     "Search",
		"resetText":      "Reset",
		"wrapWithPanel":  false,
	}, nil
}

// ── List column rendering ─────────────────────────────────────────────────────

func (r *DefaultRenderer) renderListColumns(nodes []*widget.Node, ctx renderer.RendererContext) ([]any, error) {
	var cols []any
	for _, child := range nodes {
		if child == nil || child.Hidden {
			continue
		}
		colType := nodeKindToColumnType(child.Kind)
		col := map[string]any{
			"name":  child.Name,
			"label": sanitizeText(child.Label),
			"type":  colType,
		}
		if isSortableKind(child.Kind) {
			col["sortable"] = true
		}
		if child.Kind == widget.NodeDate {
			col["format"] = "YYYY-MM-DD"
		}
		if child.Kind == widget.NodeDateTime {
			col["format"] = "YYYY-MM-DDTHH:mm:ssZ"
		}
		if child.Kind == widget.NodeBadge {
			col["type"] = "mapping"
		}
		for k, v := range child.Props {
			col[k] = v
		}
		cols = append(cols, col)
	}
	return cols, nil
}

// isSortableKind reports whether a NodeKind supports server-side column sorting
// in list view. Types with variable-length content (textarea, rich-text, file)
// are excluded as they are rarely sortable at the DB level.
func isSortableKind(kind widget.NodeKind) bool {
	switch kind {
	case widget.NodeText, widget.NodeField, widget.NodeNumber, widget.NodeMoney,
		widget.NodeDate, widget.NodeDateTime, widget.NodeSelect, widget.NodeSwitch:
		return true
	}
	return false
}

func nodeKindToColumnType(kind widget.NodeKind) string {
	switch kind {
	case widget.NodeNumber, widget.NodeMoney:
		// Use "number" not "tpl": a tpl column without a tpl template string
		// renders nothing in AMIS. The "number" column type formats the raw
		// numeric value with locale-aware separators.
		return "number"
	case widget.NodeDate:
		return "date"
	case widget.NodeDateTime:
		return "datetime"
	case widget.NodeSwitch:
		return "status"
	case widget.NodeSelect, widget.NodeMultiSelect:
		return "text"
	case widget.NodeBadge:
		return "mapping"
	case widget.NodeFileUpload, widget.NodeAttachments:
		return "file"
	default:
		return "text"
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildAPI(ds *widget.DataSource) map[string]any {
	api := map[string]any{
		"url":    ds.URL,
		"method": methodOrDefault(ds.Method),
	}
	return api
}

func buildSourceMap(ds *widget.DataSource) map[string]any {
	src := map[string]any{
		"url":    ds.URL,
		"method": methodOrDefault(ds.Method),
	}
	return src
}

func applyLabelValueFields(ds *widget.DataSource, out map[string]any) {
	if ds.LabelField != "" {
		out["labelField"] = ds.LabelField
	}
	vf := ds.ValueField
	if vf == "" {
		vf = "id"
	}
	out["valueField"] = vf
}

func methodOrDefault(method string) string {
	if method == "" {
		return "GET"
	}
	return method
}

// amisDateFormat converts a display date format string to AMIS moment.js format.
// This is a best-effort translation for common locale formats.
func amisDateFormat(displayFormat string) string {
	replacer := strings.NewReplacer(
		"DD", "DD",
		"MM", "MM",
		"YYYY", "YYYY",
		"YY", "YY",
		"HH", "HH",
		"mm", "mm",
		"ss", "ss",
	)
	return replacer.Replace(displayFormat)
}
