package workflow

import (
	"fmt"
	"strings"
)

const idSep = "."

// BuildID constructs a workflow ID following the Awo convention:
//
//	{tenantID}.{entityType}.{recordID}.{event}
//
// Example: "abc123.finance_invoice.inv456.on_submit"
func BuildID(tenantID, entityType, recordID, event string) string {
	return strings.Join([]string{tenantID, entityType, recordID, event}, idSep)
}

// ParseID decodes a workflow ID built by BuildID.
// Returns an error if the ID does not have the expected 4-segment structure.
func ParseID(id string) (tenantID, entityType, recordID, event string, err error) {
	parts := strings.SplitN(id, idSep, 4)
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("workflow.ParseID: %q: expected 4 dot-separated segments", id)
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}
