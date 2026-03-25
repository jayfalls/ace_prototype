package annot8

import (
	"log/slog"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// buildOperation turns a Chi route into an OpenAPI operation.
func (g *Generator) buildOperation(
	handler http.Handler,
	route, method string,
	middlewares []func(http.Handler) http.Handler,
) Operation {
	slog.Debug("[annot8] buildOperation: called", "route", route, "method", method)

	handlerInfo := g.extractHandlerInfo(handler, route)

	var annotations *Annotation
	if handlerInfo != nil && handlerInfo.File != "" {
		var err error
		annotations, err = ParseAnnotations(handlerInfo.File, handlerInfo.FunctionName)
		if err != nil {
			slog.Warn("[annot8] buildOperation: annotations parse error", "error", err)
		}
	}

	op := Operation{
		OperationID: generateOperationID(method, route),
		Responses:   g.buildResponses(annotations),
	}

	// Merge path parameters derived from the route itself.
	op.Parameters = append(op.Parameters, extractPathParameters(route)...)

	// Apply annotation-derived metadata.
	if annotations != nil {
		op.Summary = annotations.Summary
		op.Description = annotations.Description
		op.Tags = append(op.Tags, annotations.Tags...)

		for _, param := range annotations.Parameters {
			if param.In == "body" {
				continue
			}
			op.Parameters = upsertParameter(op.Parameters, Parameter{
				Name:        param.Name,
				In:          param.In,
				Description: param.Description,
				Required:    param.Required,
				Schema:      g.schemaGen.GenerateSchema(param.Type),
			})
		}

		if annotations.Success != nil && annotations.Success.Description != "" {
			if success := op.Responses[strconv.Itoa(annotations.Success.StatusCode)]; success.Description == "" {
				success.Description = annotations.Success.Description
				op.Responses[strconv.Itoa(annotations.Success.StatusCode)] = success
			}
		}
	}

	if len(op.Tags) == 0 {
		op.Tags = []string{extractResourceFromRoute(route)}
	}

	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		op.RequestBody = g.buildRequestBody(annotations)
	}

	if hasJWTMiddleware(middlewares) {
		op.Security = []SecurityRequirement{{"BearerAuth": {}}}
	}

	if perms := g.resolveACLPermissions(route, method, handlerInfo, middlewares); len(perms) > 0 {
		aclInfo := "\n\nAccess control:\n- " + strings.Join(perms, "\n- ")
		if op.Description != "" {
			op.Description += aclInfo
		} else {
			op.Description = "This endpoint requires authentication." + aclInfo
		}
	}

	slog.Debug("[annot8] buildOperation: completed", "operationId", op.OperationID)
	return op
}

// buildResponses assembles HTTP responses using annotations as hints.
func (g *Generator) buildResponses(annotations *Annotation) map[string]Response {
	slog.Debug("[annot8] buildResponses: called")

	responses := make(map[string]Response)

	if annotations != nil && annotations.Success != nil {
		statusCode := strconv.Itoa(annotations.Success.StatusCode)

		schema := g.generateResponseSchema(annotations.Success.DataType)
		if annotations.Success.IsWrapped {
			props := map[string]*Schema{
				"message": {Type: "string"},
				"data":    schema,
			}

			// Only include meta if the data type is a slice (implies pagination)
			if strings.HasPrefix(strings.TrimPrefix(annotations.Success.DataType, "*"), "[]") {
				props["meta"] = &Schema{Ref: "#/components/schemas/PaginationMeta"}
			}

			schema = &Schema{
				Type:       "object",
				Required:   []string{"message"},
				Properties: props,
			}
		}

		responses[statusCode] = Response{
			Description: annotations.Success.Description,
			Content: map[string]MediaTypeObject{
				"application/json": {
					Schema: schema,
				},
			},
		}
	} else {
		responses["200"] = Response{
			Description: "Successful response",
			Content: map[string]MediaTypeObject{
				"application/json": {Schema: &Schema{Type: "object"}},
			},
		}
	}

	if annotations != nil {
		for _, failure := range annotations.Failures {
			statusCode := strconv.Itoa(failure.StatusCode)
			responses[statusCode] = Response{
				Description: failure.Description,
				Content: map[string]MediaTypeObject{
					"application/problem+json": {
						Schema: &Schema{Ref: "#/components/schemas/ProblemDetails"},
					},
				},
			}
		}
	}

	standardErrors := map[string]Response{
		"400": {Description: "Bad Request", Content: problemJSON()},
		"401": {Description: "Unauthorized", Content: problemJSON()},
		"403": {Description: "Forbidden", Content: problemJSON()},
		"404": {Description: "Not Found", Content: problemJSON()},
		"500": {Description: "Internal Server Error", Content: problemJSON()},
	}

	for code, response := range standardErrors {
		if _, exists := responses[code]; !exists {
			responses[code] = response
		}
	}

	slog.Debug("[annot8] buildResponses: completed", "response_count", len(responses))
	return responses
}

func problemJSON() map[string]MediaTypeObject {
	return map[string]MediaTypeObject{
		"application/problem+json": {
			Schema: &Schema{Ref: "#/components/schemas/ProblemDetails"},
		},
	}
}

// buildRequestBody constructs a request body definition.
func (g *Generator) buildRequestBody(annotations *Annotation) *RequestBody {
	slog.Debug("[annot8] buildRequestBody: called")

	var (
		schema      *Schema
		description = "Request body"
	)

	if annotations != nil {
		for _, param := range annotations.Parameters {
			if param.In != "body" {
				continue
			}
			slog.Debug("[annot8] buildRequestBody: found body parameter", "type", param.Type)

			schema = g.schemaGen.GenerateSchema(param.Type)
			if param.Description != "" {
				description = param.Description
			}
			break
		}
	}

	if schema == nil {
		slog.Debug("[annot8] buildRequestBody: no body parameter found, using default object schema")
		schema = &Schema{Type: "object"}
	}

	return &RequestBody{
		Description: description,
		Required:    true,
		Content: map[string]MediaTypeObject{
			"application/json": {Schema: schema},
		},
	}
}

// generateResponseSchema resolves the schema referenced by an annotation.
func (g *Generator) generateResponseSchema(dataType string) *Schema {
	slog.Debug("[annot8] generateResponseSchema: called", "dataType", dataType)

	if dataType == "" {
		return &Schema{Type: "object"}
	}

	switch {
	case strings.HasPrefix(dataType, "[]"):
		itemType := strings.TrimPrefix(dataType, "[]")
		return &Schema{
			Type:  "array",
			Items: g.schemaGen.GenerateSchema(itemType),
		}
	case strings.HasPrefix(dataType, "*"):
		return g.schemaGen.GenerateSchema(strings.TrimPrefix(dataType, "*"))
	default:
		return g.schemaGen.GenerateSchema(dataType)
	}
}

// addStandardSchemas seeds reusable schemas.
func (g *Generator) addStandardSchemas(spec *Spec) {
	slog.Debug("[annot8] addStandardSchemas: adding ProblemDetails schema")

	spec.Components.Schemas["ProblemDetails"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"type":   {Type: "string", Description: "A URI reference identifying the problem type"},
			"title":  {Type: "string", Description: "A short, human-readable summary of the problem"},
			"status": {Type: "integer", Description: "The HTTP status code"},
			"detail": {Type: "string", Description: "Detailed explanation of the problem"},
			"instance": {
				Type:        "string",
				Description: "A URI reference identifying the specific instance of the problem",
			},
		},
		Required: []string{"type", "title", "status"},
	}

	spec.Components.Schemas["PaginationMeta"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"has_next":              {Type: "boolean"},
			"next_after_id":         {Type: "string", Description: "Opaque ID for pagination"},
			"next_after_created_at": {Type: "string", Format: "date-time"},
			"limit":                 {Type: "integer"},
			"records":               {Type: "integer"},
		},
	}
}

// buildTags produces tag entries sorted for determinism.
func (g *Generator) buildTags(tagNames map[string]bool) []Tag {
	slog.Debug("[annot8] buildTags: called", "tag_count", len(tagNames))

	var tags []Tag
	for name := range tagNames {
		tags = append(tags, Tag{
			Name:        name,
			Description: capitalize(name) + " related operations",
		})
	}

	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return tags
}

// convertRouteToOpenAPIPath removes regex constraints from Chi-style parameters.
func convertRouteToOpenAPIPath(route string) string {
	parts := strings.Split(route, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			inner := strings.Trim(part, "{}")
			if colon := strings.Index(inner, ":"); colon != -1 {
				parts[i] = "{" + inner[:colon] + "}"
			}
		}
	}
	return strings.Join(parts, "/")
}

// extractPathParameters converts route parameters into OpenAPI parameters.
func extractPathParameters(route string) []Parameter {
	var params []Parameter

	for _, part := range strings.Split(route, "/") {
		if !strings.HasPrefix(part, "{") || !strings.HasSuffix(part, "}") {
			continue
		}

		paramName := strings.Trim(part, "{}")
		if colon := strings.Index(paramName, ":"); colon != -1 {
			paramName = paramName[:colon]
		}

		params = append(params, Parameter{
			Name:     paramName,
			In:       "path",
			Required: true,
			Schema:   &Schema{Type: "string"},
		})
	}

	return params
}

// generateOperationID creates a stable operation ID based on method and route.
func generateOperationID(method, route string) string {
	var cleanParts []string
	for _, part := range strings.Split(strings.Trim(route, "/"), "/") {
		if part == "" || strings.Contains(part, "{") {
			continue
		}
		cleanParts = append(cleanParts, capitalize(part))
	}
	return strings.ToLower(method) + strings.Join(cleanParts, "")
}

// extractResourceFromRoute returns the first meaningful route segment.
func extractResourceFromRoute(route string) string {
	for _, part := range strings.Split(strings.Trim(route, "/"), "/") {
		if part != "" && part != "api" && part != "v1" && !strings.Contains(part, "{") {
			return part
		}
	}
	return "default"
}

// hasJWTMiddleware inspects middleware stack for auth middleware.
func hasJWTMiddleware(middlewares []func(http.Handler) http.Handler) bool {
	for _, mw := range middlewares {
		funcName := runtime.FuncForPC(reflect.ValueOf(mw).Pointer()).Name()
		if strings.Contains(funcName, "jwt") ||
			strings.Contains(funcName, "JWT") ||
			strings.Contains(funcName, "auth") ||
			strings.Contains(funcName, "Authenticated") ||
			strings.Contains(funcName, "Can") ||
			strings.Contains(funcName, "Any") ||
			strings.Contains(funcName, "Must") ||
			strings.Contains(funcName, "IsSystemAdmin") ||
			strings.Contains(funcName, "IsTenantAdmin") ||
			strings.Contains(funcName, "IsTenant") {
			return true
		}
	}
	return false
}

// capitalize upper-cases the first rune of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// upsertParameter merges a parameter into an existing slice.
func upsertParameter(params []Parameter, p Parameter) []Parameter {
	for i, existing := range params {
		if existing.Name != p.Name || existing.In != p.In {
			continue
		}

		if p.Description != "" {
			existing.Description = p.Description
		}
		if p.Required {
			existing.Required = true
		}
		if p.Schema != nil {
			existing.Schema = p.Schema
		}
		params[i] = existing
		return params
	}

	return append(params, p)
}
