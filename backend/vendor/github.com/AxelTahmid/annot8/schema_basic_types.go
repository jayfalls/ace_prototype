// Package openapi defines helpers for OpenAPI schema generation from Go types.
package annot8

import (
	"strings"
)

/**
 * mapGoTypeToOpenAPI maps a Go type name to the corresponding OpenAPI primitive type and format.
 * JSON cannot represent int64/bigint/long values accurately, so we map them to strings.
 * JSON number type is a double-precision floating-point format which can only safely represent
 * integers up to 2^53-1. Beyond that, precision may be lost as int64 goes upto 2^63-1.
 *
 * Therefore, to avoid data loss when serializing/deserializing large integers,
 * we represent int64 and uint64 as strings in the OpenAPI schema.
 *
 * https://spec.openapis.org/registry/format/
 */
func mapGoTypeToOpenAPI(typeName string) (string, string) {
	switch typeName {
	case "int8":
		return "integer", "int8"
	case "int16":
		return "integer", "int16"
	case "int32":
		return "integer", "int32"
	case "int64":
		return "string", "int64"
	case "uint8", "byte":
		return "integer", "uint32"
	case "uint16":
		return "integer", "uint16"
	case "uint32", "rune":
		return "integer", "uint32"
	case "uint64":
		return "string", "uint64"
	case "int":
		return "integer", "int"
	case "uint":
		return "integer", "uint"
	case "float32":
		return "number", "float"
	case "float64":
		return "number", "double"
	case "bool":
		return "boolean", ""
	case "string":
		return "string", ""
	default:
		return "object", ""
	}
}

// isBasicType returns true if the Go type name denotes a primitive, array, pointer or map.
// This fast-path is used to decide whether to generate a basic or complex schema.
func isBasicType(typeName string) bool {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"rune", "byte", "float32", "float64",
		"string", "bool":
		return true
	}
	if strings.HasPrefix(typeName, "[]") || strings.HasPrefix(typeName, "*") || strings.HasPrefix(typeName, "map[") {
		return true
	}
	return false
}

// generateBasicTypeSchema returns a Schema for basic Go types (primitives, slices, pointers).
// It handles arrays and pointers by delegating to GenerateSchema for element types.
func (sg *SchemaGenerator) generateBasicTypeSchema(typeName string) *Schema {
	if strings.HasPrefix(typeName, "[]") {
		elem := strings.TrimPrefix(typeName, "[]")
		return &Schema{Type: "array", Items: sg.GenerateSchema(elem)}
	}
	if strings.HasPrefix(typeName, "*") {
		// Try to see if the pointer type is known externally first (e.g. *time.Time)
		qualified := sg.getQualifiedTypeName(typeName)
		if sg.typeIndex != nil {
			if schema, ok := sg.typeIndex.externalKnownTypes[qualified]; ok {
				return schema
			}
		}

		clean := strings.TrimPrefix(typeName, "*")
		// For basic primitives, use the new 3.1 multi-type array
		if !strings.Contains(clean, ".") && isBasicType(clean) && !strings.HasPrefix(clean, "[]") &&
			!strings.HasPrefix(clean, "map[") {
			underlyingType, underlyingFormat := mapGoTypeToOpenAPI(clean)
			schema := &Schema{
				Type: []string{underlyingType, "null"},
			}
			if underlyingFormat != "" {
				schema.Format = underlyingFormat
			}
			return schema
		}

		// For complex types or slices/maps, use anyOf to avoid type conflicts
		underlying := sg.GenerateSchema(clean)
		return &Schema{
			AnyOf: []*Schema{
				underlying,
				{Type: "null"},
			},
		}
	}
	// Fallback to mapping
	openapiType, openapiFormat := mapGoTypeToOpenAPI(typeName)
	desc := openapiType + " type" + "(" + typeName + ")"
	if openapiFormat != "" {
		desc += " with format " + openapiFormat
	}

	schema := &Schema{Type: openapiType, Description: desc}
	if openapiFormat != "" {
		schema.Format = openapiFormat
	}
	return schema
}
