// Package runtime — RuntimeRegistry.
//
// RuntimeRegistry provides O(1) lookup of compiled entity metadata by every
// identity dimension (qualified name, table, permission namespace, workflow
// namespace, cache namespace, metric namespace, event namespace, route).
//
// It is constructed once from an immutable *compiler.CompiledSchema at
// process startup and is safe for concurrent read access thereafter.
//
// No package may construct entity identifiers at runtime. All lookups must go
// through RuntimeRegistry — it is the single access point to compiled metadata
// during request handling.
package runtime

import (
	"fmt"

	"awo.so/awo/compiler"
)

// RuntimeRegistry indexes CompiledSchema for O(1) lookup by every identity
// dimension used during request processing.
type RuntimeRegistry struct {
	schema *compiler.CompiledSchema

	// Primary index — all others are aliases into the same EntitySchema values.
	byQualifiedName map[string]*compiler.EntitySchema

	// Secondary indexes — each maps a namespace string to EntitySchema.
	byTable      map[string]*compiler.EntitySchema    // TableName → EntitySchema
	byPermission map[string]*compiler.EntitySchema    // PermissionNamespace → EntitySchema
	byWorkflow   map[string]*compiler.EntitySchema    // WorkflowNamespace → EntitySchema
	byEvent      map[string]*compiler.EntitySchema    // EventNamespace → EntitySchema
	byCache      map[string]*compiler.EntitySchema    // CacheNamespace → EntitySchema
	byMetric     map[string]*compiler.EntitySchema    // MetricNamespace → EntitySchema
	byRoute      map[string]*compiler.RouteDescriptor // Path → RouteDescriptor
}

// NewRuntimeRegistry builds a RuntimeRegistry from the compiled schema.
// Call once at startup; reuse the result for the lifetime of the process.
func NewRuntimeRegistry(schema *compiler.CompiledSchema) *RuntimeRegistry {
	n := len(schema.Entities)
	r := &RuntimeRegistry{
		schema:          schema,
		byQualifiedName: make(map[string]*compiler.EntitySchema, n),
		byTable:         make(map[string]*compiler.EntitySchema, n),
		byPermission:    make(map[string]*compiler.EntitySchema, n),
		byWorkflow:      make(map[string]*compiler.EntitySchema, n),
		byEvent:         make(map[string]*compiler.EntitySchema, n),
		byCache:         make(map[string]*compiler.EntitySchema, n),
		byMetric:        make(map[string]*compiler.EntitySchema, n),
		byRoute:         make(map[string]*compiler.RouteDescriptor, len(schema.Routes)),
	}

	for i := range schema.Entities {
		es := schema.Entities[i]
		r.byQualifiedName[es.QualifiedName] = es
		r.byTable[es.TableName] = es
		r.byPermission[es.PermissionNamespace] = es
		r.byWorkflow[es.WorkflowNamespace] = es
		r.byEvent[es.EventNamespace] = es
		r.byCache[es.CacheNamespace] = es
		r.byMetric[es.MetricNamespace] = es
	}

	for i := range schema.Routes {
		rd := &schema.Routes[i]
		r.byRoute[rd.Path] = rd
	}

	return r
}

// FindEntity returns the EntitySchema for the given qualified name,
// or an error if not found.
func (r *RuntimeRegistry) FindEntity(qualifiedName string) (*compiler.EntitySchema, error) {
	es := r.byQualifiedName[qualifiedName]
	if es == nil {
		return nil, fmt.Errorf("runtime: entity %q not found in compiled schema", qualifiedName)
	}
	return es, nil
}

// FindTable returns the EntitySchema whose TableName matches.
// For system entities TableName == QualifiedName.
// For custom entities TableName == "custom_entity_records" — the first
// registered custom entity is returned; callers must further filter by
// QualifiedName when working with custom entities.
func (r *RuntimeRegistry) FindTable(tableName string) (*compiler.EntitySchema, error) {
	es := r.byTable[tableName]
	if es == nil {
		return nil, fmt.Errorf("runtime: no entity with table %q", tableName)
	}
	return es, nil
}

// FindPermission returns the EntitySchema for a Casbin permission namespace.
// PermissionNamespace equals QualifiedName.
func (r *RuntimeRegistry) FindPermission(namespace string) (*compiler.EntitySchema, error) {
	es := r.byPermission[namespace]
	if es == nil {
		return nil, fmt.Errorf("runtime: no entity with permission namespace %q", namespace)
	}
	return es, nil
}

// FindWorkflow returns the EntitySchema for a Temporal workflow namespace.
// Format: module + "." + local_name (e.g. "finance.invoice").
func (r *RuntimeRegistry) FindWorkflow(namespace string) (*compiler.EntitySchema, error) {
	es := r.byWorkflow[namespace]
	if es == nil {
		return nil, fmt.Errorf("runtime: no entity with workflow namespace %q", namespace)
	}
	return es, nil
}

// FindEvent returns the EntitySchema for a domain event namespace.
// Format: module + "." + local_name (e.g. "iam.user").
func (r *RuntimeRegistry) FindEvent(namespace string) (*compiler.EntitySchema, error) {
	es := r.byEvent[namespace]
	if es == nil {
		return nil, fmt.Errorf("runtime: no entity with event namespace %q", namespace)
	}
	return es, nil
}

// FindCache returns the EntitySchema for a Redis cache namespace.
// Format: module + ":" + local_name (e.g. "iam:user").
func (r *RuntimeRegistry) FindCache(namespace string) (*compiler.EntitySchema, error) {
	es := r.byCache[namespace]
	if es == nil {
		return nil, fmt.Errorf("runtime: no entity with cache namespace %q", namespace)
	}
	return es, nil
}

// FindMetric returns the EntitySchema for a Prometheus metric namespace.
// Format: module + "_" + local_name (e.g. "iam_user") — equals QualifiedName.
func (r *RuntimeRegistry) FindMetric(namespace string) (*compiler.EntitySchema, error) {
	es := r.byMetric[namespace]
	if es == nil {
		return nil, fmt.Errorf("runtime: no entity with metric namespace %q", namespace)
	}
	return es, nil
}

// FindRoute returns the RouteDescriptor for the given Fiber path string.
// Path must match exactly as emitted by the compiler (e.g. "/api/v1/iam/users/:id").
func (r *RuntimeRegistry) FindRoute(path string) (*compiler.RouteDescriptor, error) {
	rd := r.byRoute[path]
	if rd == nil {
		return nil, fmt.Errorf("runtime: no route descriptor for path %q", path)
	}
	return rd, nil
}

// Entities returns the ordered slice of all compiled entity schemas.
// The order matches the original registration order.
func (r *RuntimeRegistry) Entities() []*compiler.EntitySchema {
	return r.schema.Entities
}

// Routes returns all compiled route descriptors.
func (r *RuntimeRegistry) Routes() []compiler.RouteDescriptor {
	return r.schema.Routes
}

// Schema returns the underlying CompiledSchema.
func (r *RuntimeRegistry) Schema() *compiler.CompiledSchema {
	return r.schema
}
