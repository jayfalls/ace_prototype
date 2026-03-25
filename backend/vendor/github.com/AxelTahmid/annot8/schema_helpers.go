package annot8

// AddWebhook attaches a webhook path item to the specification.
func (g *Generator) AddWebhook(spec *Spec, name string, pathItem PathItem) {
	if spec.Webhooks == nil {
		spec.Webhooks = make(Webhooks)
	}
	spec.Webhooks[name] = &pathItem
}

// CreateOneOfSchema returns a schema composed with oneOf.
func CreateOneOfSchema(schemas ...*Schema) *Schema {
	return &Schema{OneOf: schemas}
}

// CreateAnyOfSchema returns a schema composed with anyOf.
func CreateAnyOfSchema(schemas ...*Schema) *Schema {
	return &Schema{AnyOf: schemas}
}

// hasType checks if the schema includes the specified OpenAPI type.
func hasType(s *Schema, typeName string) bool {
	if s.Type == nil {
		return false
	}
	switch t := s.Type.(type) {
	case string:
		return t == typeName
	case []string:
		for _, v := range t {
			if v == typeName {
				return true
			}
		}
	}
	return false
}

// primaryType returns the first non-null type if multiple types are present,
// or the single type string.
func primaryType(s *Schema) string {
	if s.Type == nil {
		return ""
	}
	switch t := s.Type.(type) {
	case string:
		return t
	case []string:
		for _, v := range t {
			if v != "null" {
				return v
			}
		}
		if len(t) > 0 {
			return t[0]
		}
	}
	return ""
}

// CreateAllOfSchema returns a schema composed with allOf.
func CreateAllOfSchema(schemas ...*Schema) *Schema {
	return &Schema{AllOf: schemas}
}

// AddSchemaExample registers a named example on the schema.
func AddSchemaExample(schema *Schema, example interface{}) {
	schema.Examples = append(schema.Examples, example)
}

// AddResponseHeader appends a header to a response definition.
func AddResponseHeader(response *Response, name string, header Header) {
	if response.Headers == nil {
		response.Headers = make(map[string]Header)
	}
	response.Headers[name] = header
}

// AddResponseLink appends a link to a response definition.
func AddResponseLink(response *Response, name string, link Link) {
	if response.Links == nil {
		response.Links = make(map[string]Link)
	}
	response.Links[name] = link
}

// SetSchemaFormat sets the schema format.
func SetSchemaFormat(schema *Schema, format string) {
	schema.Format = format
}

// SetSchemaPattern sets the schema pattern.
func SetSchemaPattern(schema *Schema, pattern string) {
	schema.Pattern = pattern
}

// SetSchemaRange sets inclusive numeric range constraints.
func SetSchemaRange(schema *Schema, min, max *float64) {
	schema.Minimum = min
	schema.Maximum = max
}

// SetSchemaStringLength sets string length constraints.
func SetSchemaStringLength(schema *Schema, minLen, maxLen *int) {
	schema.MinLength = minLen
	schema.MaxLength = maxLen
}

// SetSchemaArrayConstraints sets array constraints.
func SetSchemaArrayConstraints(schema *Schema, minItems, maxItems *int, uniqueItems *bool) {
	schema.MinItems = minItems
	schema.MaxItems = maxItems
	schema.UniqueItems = uniqueItems
}

// AddSchemaEnum appends values to the enum list.
func AddSchemaEnum(schema *Schema, values ...interface{}) {
	schema.Enum = append(schema.Enum, values...)
}

// MarkSchemaDeprecated marks a schema as deprecated.
func MarkSchemaDeprecated(schema *Schema) {
	deprecated := true
	schema.Deprecated = &deprecated
}

// MarkSchemaReadOnly marks a schema as read-only.
func MarkSchemaReadOnly(schema *Schema) {
	readOnly := true
	schema.ReadOnly = &readOnly
}

// MarkSchemaWriteOnly marks a schema as write-only.
func MarkSchemaWriteOnly(schema *Schema) {
	writeOnly := true
	schema.WriteOnly = &writeOnly
}
