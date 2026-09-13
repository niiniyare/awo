package def

// ActionDef declares a custom action route on an entity. The framework
// auto-generates the route:
//
//	POST /api/v1/entities/{entity-name}/{id}/{action-name}
//
// Standard CRUD routes are generated automatically from the EntityDefinition.
// ActionDef is only needed for operations that fall outside CRUD semantics
// (e.g. "submit", "approve", "cancel", "send_email").
type ActionDef struct {
	// Name is the stable identifier for this action. Used in the URL path,
	// CapabilityGrant action values, and Temporal workflow trigger matching.
	// Convention: lowercase, underscore-separated verb (e.g. "submit",
	// "send_email", "request_approval").
	Name string

	// Method is the HTTP method for this action route. Defaults to POST when
	// empty.
	Method ActionMethod

	// Label is the human-readable button label in the SDUI.
	Label string

	// Description is shown as a tooltip or help text in the SDUI.
	Description string

	// Permission is the permission identifier required to invoke this action.
	// Format: "{module}.{entity}.{operation}" — e.g. "finance.invoice.submit".
	// MUST NOT be a role name. Role-to-permission mapping is managed separately
	// by the IAM module. If empty, the action inherits the entity's Write
	// permission identifier.
	Permission string

	// HandlerFunc is the business logic for this action. It receives a
	// pre-resolved, permission-checked [ActionContext] and returns an
	// [ActionResult].
	//
	// The handler must not open database transactions directly — use the
	// EntityRepository provided via ActionContext.Repo for all persistence.
	HandlerFunc ActionHandlerFunc

	// ConfirmMessage is shown in the SDUI before the action is invoked,
	// prompting the user to confirm. Empty string disables the confirmation
	// dialog.
	ConfirmMessage string

	// Icon is the amis icon class for the action button in the SDUI.
	Icon string

	// Hidden removes the action button from SDUI views. The route is still
	// registered and callable via the API.
	Hidden bool

	// WorkflowEvent is set when this action should fire a WorkflowTrigger
	// with the matching EventType. The runtime executes the trigger after the
	// handler returns a non-error result.
	WorkflowEvent EventType
}

// ActionHandlerFunc is the function signature for custom action handlers.
// The framework calls this function after RBAC checks pass and the target
// record is verified to exist.
type ActionHandlerFunc func(ctx *ActionContext) (*ActionResult, error)
