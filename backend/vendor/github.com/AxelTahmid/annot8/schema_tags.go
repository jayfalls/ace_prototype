// Package annot8 provides JSON-schema tag parsing utilities.
package annot8

import (
	"strconv"
	"strings"
)

// extractJSONTag returns the JSON key name from a struct tag string.
// e.g. `json:"foo,omitempty" xml:"bar"` -> "foo".
func extractJSONTag(tag string) string {
	for _, part := range strings.Split(tag, " ") {
		if strings.HasPrefix(part, "json:") {
			value := strings.Trim(part[5:], `"`)
			if comma := strings.Index(value, ","); comma != -1 {
				return value[:comma]
			}
			return value
		}
	}
	return ""
}

// extractTag retrieves the value of a specific key from a struct tag string.
// e.g. tag="validate:\"required\" json:\"foo\"", key="validate" -> "required".
func extractTag(tag, key string) string {
	for _, part := range strings.Split(tag, " ") {
		if strings.HasPrefix(part, key+":") {
			v := strings.TrimPrefix(part, key+":")
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

// applyEnhancedTags applies OpenAPI 3.1 metadata from struct tags to a schema.
func (sg *SchemaGenerator) applyEnhancedTags(schema *Schema, tag string) {
	// Parse annot8 tag for enhanced features
	if openapiTag := extractTag(tag, "annot8"); openapiTag != "" {
		parts := strings.Split(openapiTag, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.Contains(part, "=") {
				kv := strings.SplitN(part, "=", 2)
				key, value := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
				switch key {
				case "format":
					schema.Format = value
				case "pattern":
					schema.Pattern = value
				case "example":
					schema.Example = value
				case "title":
					schema.Title = value
				case "deprecated":
					if value == "true" {
						dep := true
						schema.Deprecated = &dep
					}
				case "readOnly":
					if value == "true" {
						ro := true
						schema.ReadOnly = &ro
					}
				case "writeOnly":
					if value == "true" {
						wo := true
						schema.WriteOnly = &wo
					}
				case "minimum":
					if min, err := strconv.ParseFloat(value, 64); err == nil {
						schema.Minimum = &min
					}
				case "maximum":
					if max, err := strconv.ParseFloat(value, 64); err == nil {
						schema.Maximum = &max
					}
				case "exclusiveMinimum", "exclusiveMin":
					if min, err := strconv.ParseFloat(value, 64); err == nil {
						schema.ExclusiveMinimum = &min
					}
				case "exclusiveMaximum", "exclusiveMax":
					if max, err := strconv.ParseFloat(value, 64); err == nil {
						schema.ExclusiveMaximum = &max
					}
				case "minLength":
					if m, err := strconv.Atoi(value); err == nil {
						schema.MinLength = &m
					}
				case "maxLength":
					if m, err := strconv.Atoi(value); err == nil {
						schema.MaxLength = &m
					}
				case "minItems":
					if m, err := strconv.Atoi(value); err == nil {
						schema.MinItems = &m
					}
				case "maxItems":
					if m, err := strconv.Atoi(value); err == nil {
						schema.MaxItems = &m
					}
				case "uniqueItems":
					if value == "true" {
						ui := true
						schema.UniqueItems = &ui
					}
				case "enum":
					vals := strings.Split(value, "|")
					schema.Enum = make([]interface{}, len(vals))
					for i, v := range vals {
						schema.Enum[i] = strings.TrimSpace(v)
					}
				case "default":
					schema.Default = value
				}
			}
		}
	}

	// Parse validate tag for additional constraints
	if validateTag := extractTag(tag, "validate"); validateTag != "" {
		parts := strings.Split(validateTag, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			switch {
			case part == "email":
				schema.Format = "email"
			case part == "uuid":
				schema.Format = "uuid"
			case part == "uri", part == "url":
				schema.Format = "uri"
			case strings.HasPrefix(part, "min="):
				val := strings.TrimPrefix(part, "min=")
				if hasType(schema, "integer") || hasType(schema, "number") {
					if min, err := strconv.ParseFloat(val, 64); err == nil {
						schema.Minimum = &min
					}
				} else if hasType(schema, "string") {
					if m, err := strconv.Atoi(val); err == nil {
						schema.MinLength = &m
					}
				} else if hasType(schema, "array") {
					if m, err := strconv.Atoi(val); err == nil {
						schema.MinItems = &m
					}
				}
			case strings.HasPrefix(part, "max="):
				val := strings.TrimPrefix(part, "max=")
				if hasType(schema, "integer") || hasType(schema, "number") {
					if max, err := strconv.ParseFloat(val, 64); err == nil {
						schema.Maximum = &max
					}
				} else if hasType(schema, "string") {
					if m, err := strconv.Atoi(val); err == nil {
						schema.MaxLength = &m
					}
				} else if hasType(schema, "array") {
					if m, err := strconv.Atoi(val); err == nil {
						schema.MaxItems = &m
					}
				}
			case strings.HasPrefix(part, "exclusiveMin="):
				val := strings.TrimPrefix(part, "exclusiveMin=")
				if hasType(schema, "integer") || hasType(schema, "number") {
					if min, err := strconv.ParseFloat(val, 64); err == nil {
						schema.ExclusiveMinimum = &min
					}
				}
			case strings.HasPrefix(part, "exclusiveMax="):
				val := strings.TrimPrefix(part, "exclusiveMax=")
				if hasType(schema, "integer") || hasType(schema, "number") {
					if max, err := strconv.ParseFloat(val, 64); err == nil {
						schema.ExclusiveMaximum = &max
					}
				}
			case strings.HasPrefix(part, "len="):
				val := strings.TrimPrefix(part, "len=")
				if hasType(schema, "string") {
					if m, err := strconv.Atoi(val); err == nil {
						schema.MinLength = &m
						schema.MaxLength = &m
					}
				} else if hasType(schema, "array") {
					if m, err := strconv.Atoi(val); err == nil {
						schema.MinItems = &m
						schema.MaxItems = &m
					}
				}
			case strings.HasPrefix(part, "oneof="):
				val := strings.TrimPrefix(part, "oneof=")
				vals := strings.Split(val, " ")
				schema.Enum = make([]interface{}, len(vals))
				for i, v := range vals {
					schema.Enum[i] = v
				}
			}
		}
	}

	// Parse binding tag for additional format hints
	if bindingTag := extractTag(tag, "binding"); bindingTag != "" {
		if strings.Contains(bindingTag, "email") {
			schema.Format = "email"
		}
		if strings.Contains(bindingTag, "uuid") {
			schema.Format = "uuid"
		}
	}
}
