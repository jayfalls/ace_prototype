package annot8

import (
	"go/ast"
	"log/slog"
	"strings"
)

// convertStructToSchema converts a Go AST struct type into an OpenAPI object schema.
func (sg *SchemaGenerator) convertStructToSchema(structType *ast.StructType) *Schema {
	slog.Debug("[annot8] convertStructToSchema: called")

	var allOf []*Schema
	properties := make(map[string]*Schema)
	var required []string

	for _, field := range structType.Fields.List {
		// Ensure dependent schemas generated for the field type
		switch t := field.Type.(type) {
		case *ast.Ident:
			if t.Obj != nil && t.Obj.Kind == ast.Typ {
				qualified := sg.getQualifiedTypeName(t.Name)
				_ = sg.GenerateSchema(qualified)
			}
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Typ {
				qualified := sg.getQualifiedTypeName(ident.Name)
				_ = sg.GenerateSchema(qualified)
			}
		case *ast.SelectorExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				qualified := ident.Name + "." + t.Sel.Name
				_ = sg.GenerateSchema(qualified)
			}
		}

		if len(field.Names) == 0 {
			// Handle embedded field
			embeddedSchema := sg.convertFieldType(field.Type)
			allOf = append(allOf, embeddedSchema)
			continue
		}

		for _, nameIdent := range field.Names {
			fieldName := nameIdent.Name
			if !ast.IsExported(fieldName) {
				continue // skip unexported
			}

			// Determine JSON property name
			jsonName := fieldName
			if field.Tag != nil {
				tag := strings.Trim(field.Tag.Value, "`")
				if jsonTag := extractJSONTag(tag); jsonTag != "" && jsonTag != "-" {
					jsonName = jsonTag
				}
			}

			// Convert field type
			fieldSchema := sg.convertFieldType(field.Type)

			// Apply struct tag enhancements ONLY if not a reference schema
			// References should not have sibling properties per OpenAPI 3.1 spec
			if field.Tag != nil && fieldSchema.Ref == "" {
				tag := strings.Trim(field.Tag.Value, "`")
				sg.applyEnhancedTags(fieldSchema, tag)
			}

			properties[jsonName] = fieldSchema

			// Determine required fields
			if !isPointerType(field.Type) && !hasOmitEmpty(field.Tag) {
				required = append(required, jsonName)
			}
		}
	}

	if len(allOf) == 0 {
		return &Schema{
			Type:       "object",
			Properties: properties,
			Required:   required,
		}
	}

	// if we have local properties, add as anonymous object to allOf
	if len(properties) > 0 {
		allOf = append(allOf, &Schema{
			Type:       "object",
			Properties: properties,
			Required:   required,
		})
	}

	return &Schema{
		AllOf: allOf,
	}
}

// convertFieldType inspects a Go AST expression and returns its OpenAPI schema representation.
// It handles identifiers, pointers, arrays, selectors, maps, and empty interfaces.
func (sg *SchemaGenerator) convertFieldType(expr ast.Expr) *Schema {
	slog.Debug("[annot8] convertFieldType: called")

	switch t := expr.(type) {
	case *ast.Ident:
		// Basic Go types
		basicType, basicFormat := mapGoTypeToOpenAPI(t.Name)
		if basicType != "object" {
			schema := &Schema{Type: basicType}
			if basicFormat != "" {
				schema.Format = basicFormat
			}
			return schema
		}
		// Custom types
		qualified := sg.getQualifiedTypeName(t.Name)
		return sg.GenerateSchema(qualified)

	case *ast.StarExpr:
		// Pointer types: check for external types first (like *time.Time)
		if ident, ok := t.X.(*ast.Ident); ok {
			qualified := "*" + sg.getQualifiedTypeName(ident.Name)
			if sg.typeIndex != nil {
				if schema, ok := sg.typeIndex.externalKnownTypes[qualified]; ok {
					return schema
				}
			}
		} else if sel, ok := t.X.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				qualified := "*" + ident.Name + "." + sel.Sel.Name
				if sg.typeIndex != nil {
					if schema, ok := sg.typeIndex.externalKnownTypes[qualified]; ok {
						return schema
					}
				}
			}
		}

		// Fallback: Pointer types: wrap with nullability to support OAS 3.1
		underlying := sg.convertFieldType(t.X)
		if underlying.Ref != "" {
			// For references, we use anyOf to avoid type conflicts (e.g. if the ref is a string enum)
			return &Schema{
				AnyOf: []*Schema{
					underlying,
					{Type: "null"},
				},
			}
		}

		if tStr, ok := underlying.Type.(string); ok {
			underlying.Type = []string{tStr, "null"}
		}
		return underlying

	case *ast.ArrayType:
		// Arrays and slices
		elem := sg.convertFieldType(t.Elt)
		return &Schema{Type: "array", Items: elem}

	case *ast.SelectorExpr:
		// Qualified types (e.g., time.Time)
		if ident, ok := t.X.(*ast.Ident); ok {
			qualified := ident.Name + "." + t.Sel.Name
			return sg.GenerateSchema(qualified)
		}

	case *ast.MapType:
		// Maps as object with additionalProperties
		return &Schema{Type: "object", AdditionalProperties: sg.convertFieldType(t.Value)}

	case *ast.InterfaceType:
		// Empty interface as object
		return &Schema{Type: "object"}
	}

	slog.Debug("[annot8] convertFieldType: unknown type, defaulting to object")
	return &Schema{Type: "object"}
}

// isPointerType returns true if the given AST expression represents a pointer type.
func isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// hasOmitEmpty reports whether the struct field tag includes the "omitempty" option.
func hasOmitEmpty(tag *ast.BasicLit) bool {
	if tag == nil {
		return false
	}
	return strings.Contains(tag.Value, "omitempty")
}
