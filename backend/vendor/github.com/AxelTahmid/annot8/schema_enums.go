// Package annot8 provides enum detection and schema generation for string-based Go enums.
package annot8

import (
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"strings"
)

// handleEnumType checks if a qualified Go type is a string-based enum and generates a schema with enum values.
func (sg *SchemaGenerator) handleEnumType(qualifiedName string) *Schema {
	slog.Debug("[annot8] handleEnumType: checking enum type", "qualifiedName", qualifiedName)
	if sg.typeIndex == nil {
		return nil
	}

	ts := sg.typeIndex.LookupQualifiedType(qualifiedName)
	if ts == nil {
		return nil
	}

	// String-based alias enums
	if ident, ok := ts.Type.(*ast.Ident); ok && ident.Name == "string" {
		parts := strings.Split(qualifiedName, ".")
		if len(parts) != 2 {
			return nil
		}
		pkg, typ := parts[0], parts[1]

		enumValues := sg.extractEnumValues(pkg, typ)
		if len(enumValues) > 0 {
			return &Schema{
				Type:        "string",
				Enum:        enumValues,
				Description: fmt.Sprintf("Enum type %s", qualifiedName),
			}
		}
	}
	return nil
}

// extractEnumValues finds constant string values for a given type in AST files.
func (sg *SchemaGenerator) extractEnumValues(packageName, typeName string) []interface{} {
	slog.Debug("[annot8] extractEnumValues: extracting values", "pkg", packageName, "type", typeName)
	if sg.typeIndex == nil {
		return nil
	}

	var values []interface{}
	for _, file := range sg.typeIndex.files {
		if file.Name.Name != packageName {
			continue
		}
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
				for _, spec := range gen.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && sg.isConstantOfType(vs, typeName) {
						for i := range vs.Names {
							if i < len(vs.Values) {
								if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
									v := strings.Trim(lit.Value, `"`)
									values = append(values, v)
								}
							}
						}
					}
				}
			}
		}
	}
	return values
}

// isConstantOfType determines whether a constant ValueSpec AST node is declared as the specified type.
// Returns true if the ValueSpec.Type matches the provided typeName.
func (sg *SchemaGenerator) isConstantOfType(vs *ast.ValueSpec, typeName string) bool {
	if vs.Type == nil {
		return false
	}
	if ident, ok := vs.Type.(*ast.Ident); ok {
		return ident.Name == typeName
	}
	return false
}
