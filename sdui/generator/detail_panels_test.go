package generator_test

import (
	"strings"
	"testing"

	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

func findChild(root *widget.Node, kind widget.NodeKind, name string) *widget.Node {
	for _, c := range root.Children {
		if c.Kind == kind && c.Name == name {
			return c
		}
	}
	return nil
}

func TestDetail_AttachmentsPanelGatedByPermission(t *testing.T) {
	schema := makeSchema()
	schema.AttachmentsURL = "/api/v1/platform/attachments"

	root, err := generator.New().Generate(schema, makeCtx(sduictx.ViewModeDetail, allPermsViewer()))
	if err != nil {
		t.Fatal(err)
	}
	panel := findChild(root, widget.NodeRelatedList, "attachments")
	if panel == nil {
		t.Fatal("attachments panel missing for viewer with attachment read permission")
	}
	url := panel.DataSource.URL
	if !strings.HasPrefix(url, "/api/v1/platform/attachments?") ||
		!strings.Contains(url, "filter[entity_name][eq]=finance_invoice") ||
		!strings.Contains(url, "filter[entity_id][eq]=${id}") {
		t.Errorf("attachments URL = %q", url)
	}

	viewer := &stubViewer{perms: map[string]bool{"finance.invoice.read": true}}
	root, err = generator.New().Generate(schema, makeCtx(sduictx.ViewModeDetail, viewer))
	if err != nil {
		t.Fatal(err)
	}
	if findChild(root, widget.NodeRelatedList, "attachments") != nil {
		t.Error("attachments panel must be absent without platform.attachment.read")
	}
}

func TestDetail_ActivityUsesAuditDataAPI(t *testing.T) {
	schema := makeSchema()
	schema.HasAudit = true
	schema.AuditURL = "/api/v1/iam/audit_logs"

	root, err := generator.New().Generate(schema, makeCtx(sduictx.ViewModeDetail, allPermsViewer()))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range root.Children {
		if c.Kind == widget.NodeActivity {
			if got := c.DataSource.URL; !strings.HasPrefix(got, "/api/v1/iam/audit_logs?") {
				t.Errorf("activity URL = %q, want the audit data API", got)
			}
			return
		}
	}
	t.Fatal("activity timeline missing")
}
