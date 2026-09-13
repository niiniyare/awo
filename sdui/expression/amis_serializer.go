package expression

import (
	"fmt"
	"strings"
)

// AMISSerializer translates an ExpressionNode to an AMIS JavaScript expression
// string. AMIS evaluates these strings in the browser with form data available
// as the `data` object.
//
// Example output: "data.status === 'active' && data.amount > 0"
//
// Only the AMIS renderer package should use this serializer. The generator must
// never call this — the generator emits ExpressionRef values.
type AMISSerializer struct{}

// Serialize converts expr to an AMIS JS expression string.
// Returns an error for unknown node types.
func (s AMISSerializer) Serialize(expr ExpressionNode) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("amis_serializer: nil expression")
	}
	return s.serialize(expr)
}

func (s AMISSerializer) serialize(expr ExpressionNode) (string, error) {
	switch e := expr.(type) {
	case FieldRef:
		return "data." + e.Field, nil

	case Literal:
		return litToJS(e.Value), nil

	case Compare:
		left, err := s.serialize(e.Left)
		if err != nil {
			return "", err
		}
		right, err := s.serialize(e.Right)
		if err != nil {
			return "", err
		}
		op, err := compareOpToJS(e.Op)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s %s", left, op, right), nil

	case Logical:
		left, err := s.serialize(e.Left)
		if err != nil {
			return "", err
		}
		right, err := s.serialize(e.Right)
		if err != nil {
			return "", err
		}
		op, err := logicalOpToJS(e.Op)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", left, op, right), nil

	case Not:
		inner, err := s.serialize(e.Expr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("!(%s)", inner), nil

	case In:
		field, err := s.serialize(e.Field)
		if err != nil {
			return "", err
		}
		parts := make([]string, len(e.Values))
		for i, v := range e.Values {
			parts[i] = litToJS(v.Value)
		}
		return fmt.Sprintf("[%s].includes(%s)", strings.Join(parts, ","), field), nil

	case RawExpression:
		// Pass raw AMIS expression strings through unchanged.
		return e.Raw, nil

	default:
		return "", fmt.Errorf("amis_serializer: unknown expression type %T", expr)
	}
}

func litToJS(v any) string {
	switch val := v.(type) {
	case string:
		// Escape single quotes in string literals.
		escaped := strings.ReplaceAll(val, "'", "\\'")
		return "'" + escaped + "'"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func compareOpToJS(op CompareOp) (string, error) {
	switch op {
	case OpEq:
		return "===", nil
	case OpNeq:
		return "!==", nil
	case OpGt:
		return ">", nil
	case OpGte:
		return ">=", nil
	case OpLt:
		return "<", nil
	case OpLte:
		return "<=", nil
	default:
		return "", fmt.Errorf("amis_serializer: unknown CompareOp %q", op)
	}
}

func logicalOpToJS(op LogicalOp) (string, error) {
	switch op {
	case OpAnd:
		return "&&", nil
	case OpOr:
		return "||", nil
	default:
		return "", fmt.Errorf("amis_serializer: unknown LogicalOp %q", op)
	}
}
