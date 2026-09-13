package layout_test

// Fuzz tests for the layout engine.
//
// Malformed widget trees must never panic the layout engine.
// Run: go test -fuzz=FuzzEngine_Compute ./awo/sdui/layout/...

import (
	"testing"

	"awo.so/awo/sdui/layout"
	"awo.so/awo/sdui/widget"
)

// FuzzEngine_ColSpan fuzzes the ColSpan hint value.
// Out-of-range, negative, and extreme values must not panic.
func FuzzEngine_ColSpan(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(6)
	f.Add(12)
	f.Add(13)
	f.Add(100)
	f.Add(-1)
	f.Add(^int(0)) // MaxInt

	e := layout.New()

	f.Fuzz(func(t *testing.T, colSpan int) {
		root := &widget.Node{
			Kind: widget.NodePage,
			Children: []*widget.Node{
				{
					Kind: widget.NodeForm,
					Children: []*widget.Node{
						{
							Kind:   widget.NodeText,
							Name:   "f1",
							Layout: &widget.LayoutHint{ColSpan: colSpan},
						},
					},
				},
			},
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with ColSpan=%d: %v", colSpan, r)
				}
			}()
			_, _ = e.Compute(root)
		}()
	})
}

// FuzzEngine_SectionColumns fuzzes the section column count.
func FuzzEngine_SectionColumns(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(2)
	f.Add(4)
	f.Add(5)
	f.Add(-1)
	f.Add(1000)

	e := layout.New()

	f.Fuzz(func(t *testing.T, cols int) {
		sec := &widget.Node{
			Kind:   widget.NodeSection,
			Layout: &widget.LayoutHint{ColSpan: cols},
			Children: []*widget.Node{
				{Kind: widget.NodeText, Name: "a"},
				{Kind: widget.NodeText, Name: "b"},
			},
		}
		root := &widget.Node{
			Kind:     widget.NodePage,
			Children: []*widget.Node{form(sec)},
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with section cols=%d: %v", cols, r)
				}
			}()
			_, _ = e.Compute(root)
		}()
	})
}

// FuzzEngine_NilChildren ensures nil child nodes don't panic.
func FuzzEngine_NilChildren(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(5)

	e := layout.New()

	f.Fuzz(func(t *testing.T, nilIndex int) {
		children := []*widget.Node{
			{Kind: widget.NodeText, Name: "a"},
			nil, // explicit nil child
			{Kind: widget.NodeText, Name: "b"},
		}
		root := &widget.Node{
			Kind: widget.NodePage,
			Children: []*widget.Node{
				{Kind: widget.NodeForm, Children: children},
			},
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with nil child: %v", r)
				}
			}()
			_, _ = e.Compute(root)
		}()
	})
}
