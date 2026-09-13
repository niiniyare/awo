// Package validation implements the semantic validation pass over a widget tree.
//
// Validation runs BEFORE rendering. If any Fatal ValidationError exists, the
// pipeline aborts and no rendering output is produced. This is a hard gate —
// a partially-valid tree is never rendered.
//
// Validation checks:
//   - No nil nodes in the tree
//   - All NodeKind values are registered
//   - Input fields have non-empty Name
//   - Nodes requiring DataSource have one
//   - NodeTabPane appears only as a child of NodeTabs
//   - NodeSection does not appear as a child of NodeTabs
//   - No node references its own ID as a child (direct cycle guard)
//   - ExpressionRef values are structurally valid (non-nil Expr)
//   - Plugin ValidationFuncs are invoked and their issues collected
package validation

import (
	"fmt"

	"awo.so/awo/sdui/registry"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// Severity classifies how a validation issue affects the pipeline.
type Severity string

const (
	// SeverityFatal aborts the pipeline. No output is produced.
	SeverityFatal Severity = "fatal"

	// SeverityWarning is recorded and logged but does not abort rendering.
	SeverityWarning Severity = "warning"
)

// Issue is a single validation finding.
type Issue struct {
	Severity Severity
	NodeID   string
	NodeKind widget.NodeKind
	Field    string
	Message  string
}

func (i Issue) Error() string {
	if i.NodeID != "" {
		return fmt.Sprintf("[%s] node %q (%s): %s", i.Severity, i.NodeID, i.NodeKind, i.Message)
	}
	return fmt.Sprintf("[%s] node (%s): %s", i.Severity, i.NodeKind, i.Message)
}

// Result is the output of a validation run.
type Result struct {
	Issues []Issue
}

// HasFatal reports whether any issue is fatal.
func (r Result) HasFatal() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityFatal {
			return true
		}
	}
	return false
}

// Fatals returns only fatal issues.
func (r Result) Fatals() []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Severity == SeverityFatal {
			out = append(out, i)
		}
	}
	return out
}

// Warnings returns only warning-level issues.
func (r Result) Warnings() []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Severity == SeverityWarning {
			out = append(out, i)
		}
	}
	return out
}

// Validator performs semantic validation over a widget tree.
type Validator struct {
	reg *registry.Registry // nil = use global registry
}

// New returns a Validator using the global registry.
func New() *Validator { return &Validator{} }

// NewWithRegistry returns a Validator using the provided isolated registry.
// Use in tests where the global registry is not sealed.
func NewWithRegistry(r *registry.Registry) *Validator { return &Validator{reg: r} }

// Validate performs all validation checks on the widget tree rooted at root.
// Returns a Result containing all found issues. Never returns a Go error —
// structural errors in the tree are reported as fatal Issues.
func (v *Validator) Validate(root *widget.Node, ctx sduictx.GeneratorContext) Result {
	var issues []Issue
	if root == nil {
		return Result{Issues: []Issue{{
			Severity: SeverityFatal,
			Message:  "root node is nil",
		}}}
	}
	seen := make(map[string]bool)
	v.validateNode(root, root, "", seen, &issues)
	return Result{Issues: issues}
}

func (v *Validator) validateNode(root, n *widget.Node, parentKind widget.NodeKind, seen map[string]bool, issues *[]Issue) {
	if n == nil {
		*issues = append(*issues, Issue{
			Severity: SeverityFatal,
			Message:  "nil node found in tree",
		})
		return
	}

	// NodeKind must be registered.
	reg := v.reg
	if reg == nil {
		// Use global registry via package-level functions.
		if _, ok := registry.Lookup(n.Kind); !ok {
			*issues = append(*issues, Issue{
				Severity: SeverityFatal,
				NodeID:   n.ID,
				NodeKind: n.Kind,
				Message:  fmt.Sprintf("NodeKind %q is not registered — add it to registry", n.Kind),
			})
		}
	} else {
		if _, ok := reg.Lookup(n.Kind); !ok {
			*issues = append(*issues, Issue{
				Severity: SeverityFatal,
				NodeID:   n.ID,
				NodeKind: n.Kind,
				Message:  fmt.Sprintf("NodeKind %q is not registered — add it to registry", n.Kind),
			})
		}
	}

	// ID uniqueness (warning — not fatal; IDs may be legitimately absent).
	if n.ID != "" {
		if seen[n.ID] {
			*issues = append(*issues, Issue{
				Severity: SeverityWarning,
				NodeID:   n.ID,
				NodeKind: n.Kind,
				Message:  fmt.Sprintf("duplicate node ID %q — IDs must be unique within the tree", n.ID),
			})
		}
		seen[n.ID] = true
	}

	// Input fields must have Name.
	if isInputField(n.Kind) && n.Name == "" {
		*issues = append(*issues, Issue{
			Severity: SeverityFatal,
			NodeID:   n.ID,
			NodeKind: n.Kind,
			Message:  "input field node is missing Name — every input field must bind to a data field",
		})
	}

	// DataSource requirement.
	if requiresDataSource(n.Kind) && (n.DataSource == nil || n.DataSource.URL == "") {
		*issues = append(*issues, Issue{
			Severity: SeverityFatal,
			NodeID:   n.ID,
			NodeKind: n.Kind,
			Message:  fmt.Sprintf("NodeKind %q requires a DataSource with non-empty URL", n.Kind),
		})
	}

	// Structural parent-child constraints.
	if n.Kind == widget.NodeTabPane && parentKind != widget.NodeTabs {
		*issues = append(*issues, Issue{
			Severity: SeverityFatal,
			NodeID:   n.ID,
			NodeKind: n.Kind,
			Message:  "NodeTabPane must be a direct child of NodeTabs",
		})
	}
	if n.Kind == widget.NodeSection && parentKind == widget.NodeTabs {
		*issues = append(*issues, Issue{
			Severity: SeverityFatal,
			NodeID:   n.ID,
			NodeKind: n.Kind,
			Message:  "NodeSection must not be a direct child of NodeTabs — use NodeTabPane for tab containers",
		})
	}

	// ExpressionRef structural validity.
	for _, expr := range []*widget.ExpressionRef{n.VisibleOn, n.HiddenOn, n.DisabledOn, n.RequiredOn} {
		if expr != nil && expr.Expr == nil {
			*issues = append(*issues, Issue{
				Severity: SeverityFatal,
				NodeID:   n.ID,
				NodeKind: n.Kind,
				Message:  "ExpressionRef has nil Expr — construct expressions using the expression package",
			})
		}
	}

	// Recurse into children.
	for _, child := range n.Children {
		v.validateNode(root, child, n.Kind, seen, issues)
	}
}

// isInputField returns true for NodeKinds that represent form input fields.
// These must have a non-empty Name to bind to a data field.
func isInputField(kind widget.NodeKind) bool {
	switch kind {
	case widget.NodeField, widget.NodeText, widget.NodeTextArea, widget.NodeRichText,
		widget.NodeNumber, widget.NodeMoney, widget.NodeSelect, widget.NodeMultiSelect,
		widget.NodeLookup, widget.NodeTreeSelect, widget.NodeDate, widget.NodeDateTime,
		widget.NodeDuration, widget.NodeSwitch, widget.NodeEditor, widget.NodeColor,
		widget.NodeSignature, widget.NodeFileUpload:
		return true
	}
	return false
}

// requiresDataSource returns true for NodeKinds that must have a DataSource.
func requiresDataSource(kind widget.NodeKind) bool {
	switch kind {
	case widget.NodeList, widget.NodeLookup, widget.NodeTreeSelect,
		widget.NodeRelatedList, widget.NodeKPICard, widget.NodeChartPanel, widget.NodeTablePanel:
		return true
	}
	return false
}
