// Package expression defines a portable expression DSL for the SDUI framework.
//
// Expressions in widget nodes (VisibleOn, HiddenOn, DisabledOn, RequiredOn)
// are stored as ExpressionNode values — a typed Go AST — not as renderer-specific
// strings. Each renderer translates ExpressionNode to its own target language:
//   - AMIS renderer: translates to JavaScript (e.g., "data.status === 'active'")
//   - Flutter renderer: translates to Dart condition logic
//   - PDF renderer: ignores most expressions (static output)
//
// This design ensures that a change to the expression logic requires changing
// only the generator, not every renderer. Renderer-specific expression strings
// are never stored in widget.Node.
//
// Construction example:
//
//	// "status == 'active' AND amount > 0"
//	expr := expression.And(
//	    expression.Eq(expression.Field("status"), expression.Lit("active")),
//	    expression.Gt(expression.Field("amount"), expression.Lit(0)),
//	)
package expression

import "fmt"

// ExpressionNode is the sealed interface for all expression AST nodes.
// All concrete types in this package implement ExpressionNode.
// External packages should not implement this interface — use the constructor
// functions (Field, Lit, Eq, Gt, And, etc.) to build expressions.
type ExpressionNode interface {
	// expressionNode is a marker method that seals this interface.
	expressionNode()

	// String returns a human-readable representation for debugging.
	// This is NOT a serialised form — do not use for rendering.
	String() string
}

// ── Leaf nodes ────────────────────────────────────────────────────────────────

// FieldRef references a field value from the current form/record data context.
// In AMIS: serialises to "data.{Field}".
type FieldRef struct {
	// Field is the data field name (e.g., "status", "amount").
	Field string
}

func (f FieldRef) expressionNode() {}
func (f FieldRef) String() string  { return fmt.Sprintf("field(%s)", f.Field) }

// Literal is a literal constant value (string, number, bool, nil).
// Construct via the Lit() function.
type Literal struct {
	// Value is the constant. Must be a JSON-serialisable primitive.
	Value any
}

func (l Literal) expressionNode() {}
func (l Literal) String() string  { return fmt.Sprintf("lit(%v)", l.Value) }

// ── Comparison nodes ──────────────────────────────────────────────────────────

// CompareOp is a binary comparison operator.
type CompareOp string

const (
	OpEq  CompareOp = "eq"  // ==
	OpNeq CompareOp = "neq" // !=
	OpGt  CompareOp = "gt"  // >
	OpGte CompareOp = "gte" // >=
	OpLt  CompareOp = "lt"  // <
	OpLte CompareOp = "lte" // <=
)

// Compare is a binary comparison between two expression nodes.
// Example: Compare{FieldRef{"status"}, Lit{"active"}, OpEq}
type Compare struct {
	Left  ExpressionNode
	Right ExpressionNode
	Op    CompareOp
}

func (c Compare) expressionNode() {}
func (c Compare) String() string {
	return fmt.Sprintf("(%s %s %s)", c.Left.String(), c.Op, c.Right.String())
}

// ── Logical nodes ─────────────────────────────────────────────────────────────

// LogicalOp is a binary logical operator.
type LogicalOp string

const (
	OpAnd LogicalOp = "and" // &&
	OpOr  LogicalOp = "or"  // ||
)

// Logical combines two expressions with a logical operator.
type Logical struct {
	Left  ExpressionNode
	Right ExpressionNode
	Op    LogicalOp
}

func (l Logical) expressionNode() {}
func (l Logical) String() string {
	return fmt.Sprintf("(%s %s %s)", l.Left.String(), l.Op, l.Right.String())
}

// Not negates an expression.
type Not struct {
	Expr ExpressionNode
}

func (n Not) expressionNode() {}
func (n Not) String() string  { return fmt.Sprintf("not(%s)", n.Expr.String()) }

// In tests whether a field value appears in a list of literals.
// In AMIS: serialises to "['a','b'].includes(data.field)".
type In struct {
	Field  FieldRef
	Values []Literal
}

func (i In) expressionNode() {}
func (i In) String() string  { return fmt.Sprintf("in(%s, %v)", i.Field.String(), i.Values) }

// ── Constructor functions ─────────────────────────────────────────────────────

// Field constructs a FieldRef for the named data field.
func Field(name string) FieldRef { return FieldRef{Field: name} }

// Lit constructs a Literal constant value.
func Lit(v any) Literal { return Literal{Value: v} }

// Eq constructs a Compare with OpEq.
func Eq(left, right ExpressionNode) Compare { return Compare{Left: left, Right: right, Op: OpEq} }

// Neq constructs a Compare with OpNeq.
func Neq(left, right ExpressionNode) Compare { return Compare{Left: left, Right: right, Op: OpNeq} }

// Gt constructs a Compare with OpGt.
func Gt(left, right ExpressionNode) Compare { return Compare{Left: left, Right: right, Op: OpGt} }

// Gte constructs a Compare with OpGte.
func Gte(left, right ExpressionNode) Compare { return Compare{Left: left, Right: right, Op: OpGte} }

// Lt constructs a Compare with OpLt.
func Lt(left, right ExpressionNode) Compare { return Compare{Left: left, Right: right, Op: OpLt} }

// Lte constructs a Compare with OpLte.
func Lte(left, right ExpressionNode) Compare { return Compare{Left: left, Right: right, Op: OpLte} }

// And combines two expressions with logical AND.
func And(left, right ExpressionNode) Logical { return Logical{Left: left, Right: right, Op: OpAnd} }

// Or combines two expressions with logical OR.
func Or(left, right ExpressionNode) Logical { return Logical{Left: left, Right: right, Op: OpOr} }

// Negate wraps an expression in Not.
func Negate(expr ExpressionNode) Not { return Not{Expr: expr} }

// IsIn constructs an In expression.
func IsIn(field FieldRef, values ...any) In {
	lits := make([]Literal, len(values))
	for i, v := range values {
		lits[i] = Literal{Value: v}
	}
	return In{Field: field, Values: lits}
}

// ── Escape hatch ──────────────────────────────────────────────────────────────

// RawExpression carries a renderer-specific expression string verbatim.
// It is the bridge between AMIS expression strings declared in def.FieldDef
// (e.g. "data.status === 'active'") and the widget IR ExpressionRef type.
//
// AMIS renderer: emits Raw unchanged.
// Other renderers: may ignore or convert RawExpression as appropriate.
//
// Use sparingly — prefer typed expression constructors (Eq, And, etc.) for
// portable expressions. Use RawExpression only for renderer-specific strings
// that cannot be represented in the typed AST.
type RawExpression struct {
	// Raw is the renderer-specific expression string.
	Raw string
}

func (r RawExpression) expressionNode() {}
func (r RawExpression) String() string  { return fmt.Sprintf("raw(%q)", r.Raw) }

// Raw constructs a RawExpression from an AMIS expression string.
// Returns nil when s is empty — callers can use this directly as *widget.ExpressionRef.
func RawExpr(s string) ExpressionNode { return RawExpression{Raw: s} }
