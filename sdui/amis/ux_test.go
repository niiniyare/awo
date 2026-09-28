package amis_test

import (
	"testing"

	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/widget"
)

func TestList_ToolbarAndEmptyState(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{Kind: widget.NodeList, Children: []*widget.Node{
		{Kind: widget.NodeText, Name: "name", Label: "Name"},
	}})

	toolbar, _ := out["headerToolbar"].([]any)
	have := map[any]bool{}
	for _, it := range toolbar {
		have[it] = true
	}
	for _, want := range []string{"reload", "columns-toggler", "export-csv"} {
		if !have[want] {
			t.Errorf("headerToolbar missing %q: %v", want, toolbar)
		}
	}
	if out["placeholder"] != "No records found." {
		t.Errorf("placeholder = %v", out["placeholder"])
	}
	if out["columnsTogglable"] != "auto" || out["filterTogglable"] != true || out["keepItemSelectionOnPageChange"] != true {
		t.Errorf("list UX props missing: %v", out)
	}
}

func TestForm_PromptPageLeaveOnlyWhenSubmittable(t *testing.T) {
	t.Parallel()
	editable := render(t, &widget.Node{Kind: widget.NodeForm, DataSource: &widget.DataSource{URL: "/api/x", Method: "POST"}})
	if editable["promptPageLeave"] != true {
		t.Errorf("editable form must set promptPageLeave: %v", editable)
	}
	detail := render(t, &widget.Node{Kind: widget.NodeForm, DataSource: &widget.DataSource{ReadURL: "/api/x/1"}})
	if _, ok := detail["promptPageLeave"]; ok {
		t.Errorf("read-only detail form must not set promptPageLeave: %v", detail)
	}
}

func TestMessage_FallsBackToEnglish(t *testing.T) {
	t.Parallel()
	for _, loc := range []string{"", "en-US", "xx-YY", "so-SO"} {
		if got := renderer.Message(loc, renderer.MsgEmptyState); got != "No records found." {
			t.Errorf("Message(%q) = %q", loc, got)
		}
	}
	if got := renderer.Message("en", "unknown_key"); got != "unknown_key" {
		t.Errorf("unknown key = %q", got)
	}
}
