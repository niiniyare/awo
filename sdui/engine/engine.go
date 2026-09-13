// Package engine is the top-level SDUI orchestrator.
//
// The engine owns the complete SDUI request lifecycle. HTTP handlers call
// engine.Handle — they contain no SDUI business logic.
//
// # Pipeline
//
//	Request (GeneratorContext + EntitySchema)
//	  ↓
//	L3 cache lookup (rendered output)      ← cache hit returns early
//	  ↓
//	L2 cache lookup (widget tree)          ← partial cache hit skips generation
//	  ↓
//	Generator.Generate                     ← EntitySchema → *widget.Node
//	  ↓
//	L2 cache store
//	  ↓
//	Validation (hard gate)                 ← fatal issues abort pipeline
//	  ↓
//	Layout engine                          ← compute column spans / rows
//	  ↓
//	Renderer.Render                        ← *widget.Node → RenderedOutput
//	  ↓
//	L3 cache store
//	  ↓
//	Response
//
// # Thread safety
//
// Engine is safe for concurrent use after construction. All state is either
// immutable or protected by the underlying subsystems (cache singleflight,
// plugin pipeline).
//
// # Construction
//
//	eng := engine.New(engine.Options{
//	    Generator: myGenerator,
//	    Validator: validation.New(),
//	    Layout:    layout.New(),
//	    Cache:     myCache,
//	    Renderers: map[string]renderer.Renderer{"amis": amis.New()},
//	    Obs:       myObs,
//	})
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"awo.so/awo/sdui/cache"
	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/layout"
	"awo.so/awo/sdui/observability"
	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/validation"
	"awo.so/awo/sdui/widget"
)

// Request carries all inputs required to produce a rendered SDUI page.
type Request struct {
	// Ctx is the immutable generation context (tenant, viewer, view mode, locale).
	Ctx sduictx.GeneratorContext

	// Schema describes the entity to render.
	Schema generator.EntitySchema

	// RendererID selects which registered renderer to use (e.g. "amis").
	// Falls back to the engine's default renderer if empty.
	RendererID string

	// RendererCtx carries renderer-specific hints (theme, locale format, etc.).
	RendererCtx renderer.RendererContext
}

// Response is the output of a successful Handle call.
type Response struct {
	// Output is the rendered result.
	Output renderer.RenderedOutput

	// Layout is the computed layout metadata. Nil when the response was served
	// entirely from the L3 cache (no layout computation was needed).
	Layout *layout.ComputedLayout

	// CacheHit is true when the response came from a cache layer without
	// running the full generation+render pipeline.
	CacheHit bool

	// GeneratedAt is the time the response was produced.
	GeneratedAt time.Time
}

// Options configures the Engine. Generator, Validator, Layout, and at least
// one Renderer are required.
type Options struct {
	// Generator produces widget trees from EntitySchema values.
	Generator *generator.EntityGenerator

	// Validator validates widget trees before rendering.
	Validator *validation.Validator

	// Layout computes renderer-independent layout metadata.
	Layout *layout.Engine

	// Cache provides L2 (widget tree) and L3 (rendered output) caching.
	// Nil disables caching.
	Cache *cache.Cache

	// Renderers maps renderer ID → Renderer. Must contain at least one entry.
	Renderers map[string]renderer.Renderer

	// DefaultRendererID is used when Request.RendererID is empty.
	// If unset, an arbitrary registered renderer is chosen.
	DefaultRendererID string

	// Obs provides metrics and tracing. Defaults to Noop if nil.
	Obs *observability.Metrics
}

// Engine orchestrates the complete SDUI request pipeline.
// Construct via New(). Safe for concurrent use.
type Engine struct {
	gen       *generator.EntityGenerator
	validator *validation.Validator
	layout    *layout.Engine
	cache     *cache.Cache
	renderers map[string]renderer.Renderer
	defaultID string
	obs       *observability.Metrics
}

// New validates opts and returns a ready-to-use Engine.
// Panics if required fields are missing.
func New(opts Options) *Engine {
	if opts.Generator == nil {
		panic("sdui/engine: Options.Generator is required")
	}
	if opts.Validator == nil {
		panic("sdui/engine: Options.Validator is required")
	}
	if opts.Layout == nil {
		panic("sdui/engine: Options.Layout is required")
	}
	if len(opts.Renderers) == 0 {
		panic("sdui/engine: Options.Renderers must contain at least one renderer")
	}

	obs := opts.Obs
	if obs == nil {
		obs = observability.Noop()
	}

	defaultID := opts.DefaultRendererID
	if defaultID == "" {
		for id := range opts.Renderers {
			defaultID = id
			break
		}
	}

	return &Engine{
		gen:       opts.Generator,
		validator: opts.Validator,
		layout:    opts.Layout,
		cache:     opts.Cache,
		renderers: opts.Renderers,
		defaultID: defaultID,
		obs:       obs,
	}
}

// Handle executes the full SDUI pipeline for req and returns the rendered output.
//
// Errors are returned as-is; callers can distinguish validation failures from
// generation/render failures by type-asserting if needed.
func (e *Engine) Handle(ctx context.Context, req Request) (*Response, error) {
	ctx, finish := e.obs.StartSpan(ctx, "request")
	defer finish()

	// Resolve renderer.
	rendID := req.RendererID
	if rendID == "" {
		rendID = e.defaultID
	}
	rend, ok := e.renderers[rendID]
	if !ok {
		return nil, fmt.Errorf("sdui/engine: unknown renderer %q", rendID)
	}

	// Build cache key base (shared between L2 and L3, differ only by Level).
	keyBase := cache.KeyParams{
		EntityName:      req.Ctx.EntityName,
		ViewMode:        string(req.Ctx.ViewMode),
		RendererID:      rend.ID(),
		RendererVersion: rend.Version(),
		Locale:          req.Ctx.EffectiveLocale(),
		TenantIDHash:    cache.HashTenantID(req.Ctx.TenantID.String()),
		SchemaFP:        req.Ctx.SchemaFingerprint,
		PermFP:          req.Ctx.PermFingerprint,
	}

	// ── L3 cache lookup ───────────────────────────────────────────────────────
	if e.cache != nil {
		l3Key := cacheKey(keyBase, "l3")
		spanCtx, spanEnd := e.obs.StartSpan(ctx, "cache_lookup")
		raw, err := e.cache.GetRenderedOutput(spanCtx, l3Key)
		spanEnd()
		if err == nil && raw != nil {
			e.obs.RecordCacheHit(ctx, observability.CacheLevelL3)
			var out renderer.RenderedOutput
			if jsonErr := json.Unmarshal(raw, &out); jsonErr == nil {
				return &Response{Output: out, CacheHit: true, GeneratedAt: time.Now()}, nil
			}
			// Corrupt entry — fall through.
		} else {
			e.obs.RecordCacheMiss(ctx, observability.CacheLevelL3)
		}
	}

	// ── L2 cache lookup ───────────────────────────────────────────────────────
	var root *widget.Node
	if e.cache != nil {
		l2Key := cacheKey(keyBase, "l2")
		spanCtx, spanEnd := e.obs.StartSpan(ctx, "cache_lookup")
		raw, err := e.cache.GetWidgetTree(spanCtx, l2Key)
		spanEnd()
		if err == nil && raw != nil {
			e.obs.RecordCacheHit(ctx, observability.CacheLevelL2)
			var n widget.Node
			if jsonErr := json.Unmarshal(raw, &n); jsonErr == nil {
				root = &n
			}
			// Corrupt entry → root stays nil → regenerate.
		} else {
			e.obs.RecordCacheMiss(ctx, observability.CacheLevelL2)
		}
	}

	// ── Generation ────────────────────────────────────────────────────────────
	if root == nil {
		genTimer := e.obs.TrackGeneration(ctx, req.Ctx.EntityName, string(req.Ctx.ViewMode))
		_, spanEnd := e.obs.StartSpan(ctx, "generate")
		var err error
		root, err = e.gen.Generate(req.Schema, req.Ctx)
		spanEnd()
		genTimer.Done()
		if err != nil {
			e.obs.RecordError(ctx, observability.StageGenerate)
			return nil, fmt.Errorf("sdui/engine: generation: %w", err)
		}

		// Store in L2 cache.
		if e.cache != nil {
			l2Key := cacheKey(keyBase, "l2")
			if raw, jsonErr := json.Marshal(root); jsonErr == nil {
				_ = e.cache.SetWidgetTree(ctx, l2Key, raw)
			}
		}
	}

	// ── Validation (hard gate) ────────────────────────────────────────────────
	valTimer := e.obs.TrackValidation(ctx)
	_, spanEnd := e.obs.StartSpan(ctx, "validate")
	result := e.validator.Validate(root, req.Ctx)
	spanEnd()
	valTimer.Done()
	if result.HasFatal() {
		e.obs.RecordError(ctx, observability.StageValidate)
		fatals := result.Fatals()
		return nil, fmt.Errorf("sdui/engine: validation: %d fatal issue(s): %s",
			len(fatals), fatals[0].Message)
	}

	// ── Layout ────────────────────────────────────────────────────────────────
	layoutTimer := e.obs.TrackLayout(ctx)
	_, spanEnd = e.obs.StartSpan(ctx, "layout")
	cl, err := e.layout.Compute(root)
	spanEnd()
	layoutTimer.Done()
	if err != nil {
		e.obs.RecordError(ctx, observability.StageLayout)
		return nil, fmt.Errorf("sdui/engine: layout: %w", err)
	}

	// ── Render ────────────────────────────────────────────────────────────────
	renderTimer := e.obs.TrackRender(ctx, rend.ID())
	_, spanEnd = e.obs.StartSpan(ctx, "render")
	out, err := rend.Render(root, req.RendererCtx)
	spanEnd()
	renderTimer.Done()
	if err != nil {
		e.obs.RecordError(ctx, observability.StageRender)
		return nil, fmt.Errorf("sdui/engine: render: %w", err)
	}

	// ── L3 cache store ────────────────────────────────────────────────────────
	if e.cache != nil {
		l3Key := cacheKey(keyBase, "l3")
		if raw, jsonErr := json.Marshal(out); jsonErr == nil {
			_ = e.cache.SetRenderedOutput(ctx, l3Key, raw)
		}
	}

	return &Response{
		Output:      out,
		Layout:      cl,
		CacheHit:    false,
		GeneratedAt: time.Now(),
	}, nil
}

// Renderers returns a snapshot of the registered renderer map.
func (e *Engine) Renderers() map[string]renderer.Renderer {
	out := make(map[string]renderer.Renderer, len(e.renderers))
	for k, v := range e.renderers {
		out[k] = v
	}
	return out
}

// cacheKey builds a cache key from the base params and a level string.
func cacheKey(base cache.KeyParams, level string) string {
	p := base
	p.Level = level
	return cache.Key(p)
}
