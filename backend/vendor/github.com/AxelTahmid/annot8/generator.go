package annot8

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

// Generator produces OpenAPI specifications by analysing Chi routers.
type Generator struct {
	schemaGen     *SchemaGenerator
	handlerCache  map[uintptr]*HandlerInfo
	cacheMu       sync.RWMutex
	aclSlugOnce   sync.Once
	aclSlugMap    map[string]string
	modelNameFunc ModelNameFunc
}

// ModelNameFunc defines a strategy for converting Go package and type names into OpenAPI model names.
type ModelNameFunc func(pkg, name string) string

// DefaultModelNameFunc returns names in "pkg.Type" format (existing default).
func DefaultModelNameFunc(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// NewGeneratorWithCache creates a generator that reuses a pre-built TypeIndex.
func NewGeneratorWithCache(typeIndex *TypeIndex) *Generator {
	return &Generator{
		schemaGen: &SchemaGenerator{
			schemas:   make(map[string]*Schema),
			typeIndex: typeIndex,
		},
		handlerCache:  make(map[uintptr]*HandlerInfo),
		modelNameFunc: DefaultModelNameFunc,
	}
}

// NewGenerator creates a generator using the shared global TypeIndex.
func NewGenerator() *Generator {
	ensureTypeIndex()
	return NewGeneratorWithCache(typeIndex)
}

// SetModelNameFunc sets a custom strategy for naming OpenAPI models.
func (g *Generator) SetModelNameFunc(f ModelNameFunc) {
	g.modelNameFunc = f
}

// GenerateSchema manually adds a type to the internal schema generator.
// This is useful for including types that are not automatically discovered via routes.
func (g *Generator) GenerateSchema(typeName string) *Schema {
	return g.schemaGen.GenerateSchema(typeName)
}

// GenerateSpec assembles an OpenAPI specification for the supplied router.
func (g *Generator) GenerateSpec(router chi.Router, cfg Config) Spec {
	if cfg.Title == "" || cfg.Version == "" {
		slog.Warn("[annot8] GenerateSpec: missing required config", "title", cfg.Title, "version", cfg.Version)
	}

	slog.Debug("[annot8] GenerateSpec: called", "title", cfg.Title, "version", cfg.Version)

	spec := Spec{
		OpenAPI:           "3.1.0",
		JSONSchemaDialect: "https://spec.openapis.org/oas/3.1/dialect/base",
		Info: Info{
			Title:          cfg.Title,
			Summary:        cfg.Summary,
			Version:        cfg.Version,
			Description:    cfg.Description,
			TermsOfService: cfg.TermsOfService,
			Contact:        cfg.Contact,
			License:        cfg.License,
		},
		Paths: make(map[string]PathItem),
		Components: &Components{
			Schemas:         make(map[string]Schema),
			SecuritySchemes: make(map[string]SecurityScheme),
			Responses:       make(map[string]Response),
			Parameters:      make(map[string]Parameter),
			Examples:        make(map[string]Example),
			RequestBodies:   make(map[string]RequestBody),
			Headers:         make(map[string]Header),
			Links:           make(map[string]Link),
			Callbacks:       make(map[string]Callback),
			PathItems:       make(map[string]PathItem),
		},
	}

	if len(cfg.Servers) > 0 {
		slog.Debug("[annot8] GenerateSpec: adding servers", "count", len(cfg.Servers))
		spec.Servers = make([]Server, len(cfg.Servers))
		for i, s := range cfg.Servers {
			spec.Servers[i] = Server{URL: s, Description: "API Server"}
		}
	}

	spec.Components.SecuritySchemes["BearerAuth"] = SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "JWT token authentication",
	}

	g.addStandardSchemas(&spec)

	tags := make(map[string]bool)
	routes, err := DiscoverRoutes(router)
	if err != nil {
		slog.Warn("[annot8] GenerateSpec: InspectRoutes error", "error", err)
	}

	for _, ri := range routes {
		method := ri.Method
		route := ri.Pattern
		handler := ri.HandlerFunc
		pathKey := convertRouteToOpenAPIPath(route)

		operation := g.buildOperation(handler, route, method, ri.Middlewares)

		pathItem := spec.Paths[pathKey]
		switch strings.ToUpper(method) {
		case "GET":
			pathItem.Get = &operation
		case "POST":
			pathItem.Post = &operation
		case "PUT":
			pathItem.Put = &operation
		case "DELETE":
			pathItem.Delete = &operation
		case "PATCH":
			pathItem.Patch = &operation
		case "HEAD":
			pathItem.Head = &operation
		case "OPTIONS":
			pathItem.Options = &operation
		case "TRACE":
			pathItem.Trace = &operation
		}
		spec.Paths[pathKey] = pathItem

		for _, tag := range operation.Tags {
			tags[tag] = true
		}
	}

	spec.Tags = g.buildTags(tags)

	// Post-process schemas to apply the naming strategy and resolve conflicts
	g.finalizeSchemas(&spec)

	slog.Debug("[annot8] GenerateSpec: completed", "path_count", len(spec.Paths))
	return spec
}

// finalizeSchemas applies the naming strategy and resolves conflicts.
func (g *Generator) finalizeSchemas(spec *Spec) {
	schemas := g.schemaGen.GetSchemas()
	if len(schemas) == 0 {
		return
	}

	// 1. Calculate final names and resolve conflicts
	nameMap := make(map[string]string) // internalID -> finalName
	usedNames := make(map[string]int)  // finalName -> count for suffixing

	// Sort internal IDs for deterministic conflict resolution
	internalIDs := make([]string, 0, len(schemas))
	for id := range schemas {
		internalIDs = append(internalIDs, id)
	}
	sort.Strings(internalIDs)

	for _, id := range internalIDs {
		pkg, name := splitQualifiedName(id)
		final := g.modelNameFunc(pkg, name)

		if count, exists := usedNames[final]; exists {
			usedNames[final] = count + 1
			final = fmt.Sprintf("%s%d", final, usedNames[final])
		} else {
			usedNames[final] = 1
		}
		nameMap[id] = final
	}

	// 2. Transfer schemas to spec with new names
	for oldID, schema := range schemas {
		newName := nameMap[oldID]
		spec.Components.Schemas[newName] = schema
	}

	// 3. Update all $ref in the entire specification
	refMapping := make(map[string]string)
	for oldID, newName := range nameMap {
		oldRef := fmt.Sprintf("#/components/schemas/%s", oldID)
		newRef := fmt.Sprintf("#/components/schemas/%s", newName)
		refMapping[oldRef] = newRef
	}

	g.updateRefs(spec, refMapping)
}

// splitQualifiedName splits "pkg.Name" into ("pkg", "Name").
func splitQualifiedName(id string) (string, string) {
	idx := strings.LastIndex(id, ".")
	if idx == -1 {
		return "", id
	}
	return id[:idx], id[idx+1:]
}

// updateRefs recursively traverses the spec and replaces $ref values.
func (g *Generator) updateRefs(spec *Spec, mapping map[string]string) {
	// Update all schemas in components
	for name := range spec.Components.Schemas {
		s := spec.Components.Schemas[name]
		g.updateSchemaRefs(&s, mapping)
		spec.Components.Schemas[name] = s
	}

	// Update all paths
	for path := range spec.Paths {
		pi := spec.Paths[path]
		g.updatePathItemRefs(&pi, mapping)
		spec.Paths[path] = pi
	}

	// Update webhooks
	for name := range spec.Webhooks {
		pi := spec.Webhooks[name]
		g.updatePathItemRefs(pi, mapping)
	}
}

func (g *Generator) updateSchemaRefs(s *Schema, mapping map[string]string) {
	if s == nil {
		return
	}

	if s.Ref != "" {
		if newRef, ok := mapping[s.Ref]; ok {
			s.Ref = newRef
		}
	}

	for k := range s.Properties {
		g.updateSchemaRefs(s.Properties[k], mapping)
	}

	if s.Items != nil {
		g.updateSchemaRefs(s.Items, mapping)
	}

	for _, sub := range s.OneOf {
		g.updateSchemaRefs(sub, mapping)
	}
	for _, sub := range s.AnyOf {
		g.updateSchemaRefs(sub, mapping)
	}
	for _, sub := range s.AllOf {
		g.updateSchemaRefs(sub, mapping)
	}

	if s.Not != nil {
		g.updateSchemaRefs(s.Not, mapping)
	}

	if ap, ok := s.AdditionalProperties.(*Schema); ok && ap != nil {
		g.updateSchemaRefs(ap, mapping)
	}
}

func (g *Generator) updatePathItemRefs(pi *PathItem, mapping map[string]string) {
	if pi == nil {
		return
	}
	g.updateOperationRefs(pi.Get, mapping)
	g.updateOperationRefs(pi.Put, mapping)
	g.updateOperationRefs(pi.Post, mapping)
	g.updateOperationRefs(pi.Delete, mapping)
	g.updateOperationRefs(pi.Options, mapping)
	g.updateOperationRefs(pi.Head, mapping)
	g.updateOperationRefs(pi.Patch, mapping)
	g.updateOperationRefs(pi.Trace, mapping)

	for i := range pi.Parameters {
		g.updateParameterRefs(&pi.Parameters[i], mapping)
	}
}

func (g *Generator) updateOperationRefs(op *Operation, mapping map[string]string) {
	if op == nil {
		return
	}
	for i := range op.Parameters {
		g.updateParameterRefs(&op.Parameters[i], mapping)
	}
	if op.RequestBody != nil {
		for k := range op.RequestBody.Content {
			mt := op.RequestBody.Content[k]
			g.updateSchemaRefs(mt.Schema, mapping)
			op.RequestBody.Content[k] = mt
		}
	}
	for k := range op.Responses {
		resp := op.Responses[k]
		for mk := range resp.Content {
			mt := resp.Content[mk]
			g.updateSchemaRefs(mt.Schema, mapping)
			resp.Content[mk] = mt
		}
		op.Responses[k] = resp
	}
	for k := range op.Callbacks {
		cb := op.Callbacks[k]
		for ck := range cb {
			pi := cb[ck]
			g.updatePathItemRefs(pi, mapping)
		}
	}
}

func (g *Generator) updateParameterRefs(p *Parameter, mapping map[string]string) {
	if p == nil {
		return
	}
	g.updateSchemaRefs(p.Schema, mapping)
}
