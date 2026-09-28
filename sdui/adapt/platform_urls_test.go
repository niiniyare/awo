package adapt_test

import (
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/platform/attachment"
	"awo.so/awo/platform/audit"
	"awo.so/awo/platform/iam"
	"awo.so/awo/registry"
	"awo.so/awo/sdui/adapt"
)

// The hard-coded audit and attachment list URLs must match the RoutePrefix the
// compiler assigns to the real platform entities.
func TestFromCompiled_PlatformURLsMatchCompiledRoutes(t *testing.T) {
	reg, err := registry.BuildFrom([]def.EntityDefinition{
		&audit.LogDefinition, &attachment.Definition, &iam.UserDefinition, &invoiceDef,
	})
	if err != nil {
		t.Fatal(err)
	}
	cs, err := compiler.Compile(reg)
	if err != nil {
		t.Fatal(err)
	}

	gs := adapt.FromCompiled(cs.ByName["test_invoice"])
	if want := cs.ByName["iam_audit_log"].RoutePrefix; gs.AuditURL != want {
		t.Errorf("AuditURL = %q, compiled route = %q", gs.AuditURL, want)
	}
	att := cs.ByName["platform_attachment"]
	if gs.AttachmentsURL != att.RoutePrefix {
		t.Errorf("AttachmentsURL = %q, compiled route = %q", gs.AttachmentsURL, att.RoutePrefix)
	}
	if adapt.FromCompiled(att).AttachmentsURL != "" {
		t.Error("attachment entity must not list its own attachments")
	}
}
