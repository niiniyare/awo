package registry_test

import (
	"testing"

	"awo.so/awo/sdui/registry"
	"awo.so/awo/sdui/widget"
)

func TestIsolatedRegistry_RegisterAndLookup(t *testing.T) {
	r := registry.NewIsolated()
	def := registry.WidgetDef{Kind: widget.NodeText, DisplayName: "Text", IsInputField: true}
	if err := r.Register(def); err != nil {
		t.Fatal(err)
	}
	got, ok := r.Lookup(widget.NodeText)
	if !ok {
		t.Fatal("expected to find NodeText")
	}
	if got.DisplayName != "Text" {
		t.Errorf("got %q, want %q", got.DisplayName, "Text")
	}
}

func TestIsolatedRegistry_DuplicateRegistration(t *testing.T) {
	r := registry.NewIsolated()
	def := registry.WidgetDef{Kind: widget.NodeText, DisplayName: "Text"}
	if err := r.Register(def); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(def); err == nil {
		t.Error("expected error on duplicate registration")
	}
}

func TestIsolatedRegistry_RegisterAfterSeal(t *testing.T) {
	r := registry.NewIsolated()
	r.Seal()
	def := registry.WidgetDef{Kind: widget.NodeText, DisplayName: "Text"}
	if err := r.Register(def); err == nil {
		t.Error("expected error when registering after seal")
	}
}

func TestIsolatedRegistry_LookupUnknown(t *testing.T) {
	r := registry.NewIsolated()
	r.Seal()
	_, ok := r.Lookup(widget.NodeKind("unknown_widget_xyz"))
	if ok {
		t.Error("expected false for unknown widget kind")
	}
}

func TestGlobalRegistry_BuiltinsRegistered(t *testing.T) {
	// The global registry is populated by builtin.go init().
	// Verify a representative sample of built-in kinds are present.
	kinds := []widget.NodeKind{
		widget.NodePage, widget.NodeForm, widget.NodeList,
		widget.NodeText, widget.NodeSelect, widget.NodeLookup,
		widget.NodeTabPane, widget.NodeGrid, widget.NodeMoney,
	}
	for _, kind := range kinds {
		if _, ok := registry.Lookup(kind); !ok {
			t.Errorf("expected built-in kind %q to be registered", kind)
		}
	}
}

func TestIsolatedRegistry_AllKinds(t *testing.T) {
	r := registry.NewIsolated()
	r.Register(registry.WidgetDef{Kind: widget.NodeText})   //nolint
	r.Register(registry.WidgetDef{Kind: widget.NodeNumber}) //nolint
	r.Seal()
	kinds := r.AllKinds()
	if len(kinds) != 2 {
		t.Errorf("expected 2 kinds, got %d", len(kinds))
	}
}

func TestIsolatedRegistry_Validate(t *testing.T) {
	r := registry.NewIsolated()
	// Register a parent that references an unregistered child.
	r.Register(registry.WidgetDef{ //nolint
		Kind:            widget.NodeForm,
		AllowedChildren: []widget.NodeKind{widget.NodeText}, // NodeText not registered
	})
	r.Seal()
	if err := r.Validate(); err == nil {
		t.Error("expected validation error for unregistered child kind")
	}
}
