package plugins_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/sdui/plugins"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// stubViewer implements sduictx.ViewerContext for tests.
type stubViewer struct{}

func (stubViewer) TenantID() uuid.UUID       { return uuid.New() }
func (stubViewer) Roles() []string           { return nil }
func (stubViewer) IsPlatformAdmin() bool     { return false }
func (stubViewer) HasPermission(string) bool { return true }

func makeCtx() sduictx.GeneratorContext {
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), stubViewer{}, "test_entity", sduictx.ViewModeList, "amis",
	).Build()
	return ctx
}

func TestPipeline_DuplicatePriority(t *testing.T) {
	p := plugins.NewIsolated()
	noop := func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
		return root, nil
	}
	if err := p.RegisterTreeTransform(plugins.ExtPostGeneration, "a", 10, noop); err != nil {
		t.Fatal(err)
	}
	if err := p.RegisterTreeTransform(plugins.ExtPostGeneration, "b", 10, noop); err == nil {
		t.Error("expected error for duplicate priority 10")
	}
}

func TestPipeline_ExecutionOrder(t *testing.T) {
	p := plugins.NewIsolated()
	var order []string

	makePlugin := func(name string) plugins.TreeTransformFunc {
		return func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
			order = append(order, name)
			return root, nil
		}
	}

	p.RegisterTreeTransform(plugins.ExtPostGeneration, "second", 20, makePlugin("second")) //nolint
	p.RegisterTreeTransform(plugins.ExtPostGeneration, "first", 10, makePlugin("first"))   //nolint
	p.Seal()                                                                               //nolint

	root := &widget.Node{Kind: widget.NodePage}
	p.RunTreeTransforms(plugins.ExtPostGeneration, root, makeCtx()) //nolint

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("unexpected execution order: %v", order)
	}
}

func TestPipeline_RecoverableError(t *testing.T) {
	p := plugins.NewIsolated()

	errPlugin := func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
		return nil, errors.New("recoverable failure")
	}
	sentinel := &widget.Node{Kind: widget.NodeForm}
	passPlugin := func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
		return sentinel, nil
	}

	p.RegisterTreeTransform(plugins.ExtPostGeneration, "err", 10, errPlugin)   //nolint
	p.RegisterTreeTransform(plugins.ExtPostGeneration, "pass", 20, passPlugin) //nolint
	p.Seal()                                                                   //nolint

	root := &widget.Node{Kind: widget.NodePage}
	result, err := p.RunTreeTransforms(plugins.ExtPostGeneration, root, makeCtx())
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	// pass plugin runs after err plugin; err plugin is skipped → pass plugin uses original root
	if result != sentinel {
		t.Error("expected sentinel from pass plugin")
	}
}

func TestPipeline_FatalError(t *testing.T) {
	p := plugins.NewIsolated()

	fatalPlugin := func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
		return nil, &plugins.PluginFatalError{
			PluginName: "fatal_plugin",
			Point:      plugins.ExtPostGeneration,
			Cause:      errors.New("data corruption detected"),
		}
	}
	p.RegisterTreeTransform(plugins.ExtPostGeneration, "fatal", 10, fatalPlugin) //nolint
	p.Seal()                                                                     //nolint

	root := &widget.Node{Kind: widget.NodePage}
	_, err := p.RunTreeTransforms(plugins.ExtPostGeneration, root, makeCtx())
	if err == nil {
		t.Fatal("expected fatal error")
	}
	if !plugins.IsFatal(err) {
		t.Errorf("expected IsFatal=true, got err=%v", err)
	}
}

func TestPipeline_RegisterAfterSeal(t *testing.T) {
	p := plugins.NewIsolated()
	p.Seal() //nolint
	noop := func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
		return root, nil
	}
	if err := p.RegisterTreeTransform(plugins.ExtPostGeneration, "late", 10, noop); err == nil {
		t.Error("expected error when registering after seal")
	}
}
