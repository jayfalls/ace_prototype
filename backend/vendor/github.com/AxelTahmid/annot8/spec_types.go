package annot8

// Config defines the configuration for OpenAPI specification generation.
// All fields except Title and Version are optional.
type Config struct {
	Title          string   // Required: API title
	Summary        string   // Optional: API summary
	Description    string   // Optional: API description
	Version        string   // Required: API version (e.g., "1.0.0")
	TermsOfService string   // Optional: Terms of service URL
	Servers        []string // Optional: List of base server URLs
	Contact        *Contact // Optional: Contact information
	License        *License // Optional: License information
}

// Contact represents contact information for the API.
type Contact struct {
	Name  string // Contact name
	URL   string // Contact URL
	Email string // Contact email address
}

// License represents license information for the API.
type License struct {
	Name       string `json:"name"` // License name (e.g., "MIT", "Apache 2.0")
	Identifier string `json:"identifier,omitempty"`
	URL        string `json:"url,omitempty"` // License URL
}

// Spec represents a complete OpenAPI 3.1 specification.
type Spec struct {
	OpenAPI           string                 `json:"openapi"`
	Info              Info                   `json:"info"`
	JSONSchemaDialect string                 `json:"jsonSchemaDialect,omitempty"`
	Servers           []Server               `json:"servers,omitempty"`
	Paths             map[string]PathItem    `json:"paths"`
	Webhooks          Webhooks               `json:"webhooks,omitempty"`
	Components        *Components            `json:"components,omitempty"`
	Tags              []Tag                  `json:"tags,omitempty"`
	Security          []SecurityRequirement  `json:"security,omitempty"`
	ExternalDocs      *ExternalDocumentation `json:"externalDocs,omitempty"`
}

// Info captures high-level metadata about the API.
type Info struct {
	Title          string   `json:"title"`
	Summary        string   `json:"summary,omitempty"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Version        string   `json:"version"`
}

// Server declares an API server entry.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem groups operations under a single route.
type PathItem struct {
	Ref         string      `json:"$ref,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Get         *Operation  `json:"get,omitempty"`
	Put         *Operation  `json:"put,omitempty"`
	Post        *Operation  `json:"post,omitempty"`
	Delete      *Operation  `json:"delete,omitempty"`
	Options     *Operation  `json:"options,omitempty"`
	Head        *Operation  `json:"head,omitempty"`
	Patch       *Operation  `json:"patch,omitempty"`
	Trace       *Operation  `json:"trace,omitempty"`
	Servers     []Server    `json:"servers,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

// Operation represents a single HTTP operation.
type Operation struct {
	Tags         []string               `json:"tags,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Description  string                 `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
	OperationID  string                 `json:"operationId,omitempty"`
	Parameters   []Parameter            `json:"parameters,omitempty"`
	RequestBody  *RequestBody           `json:"requestBody,omitempty"`
	Responses    map[string]Response    `json:"responses"`
	Callbacks    map[string]Callback    `json:"callbacks,omitempty"`
	Deprecated   bool                   `json:"deprecated,omitempty"`
	Security     []SecurityRequirement  `json:"security,omitempty"`
	Servers      []Server               `json:"servers,omitempty"`
}

// Parameter describes a path/query/header parameter.
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody describes an HTTP request payload.
type RequestBody struct {
	Description string                     `json:"description,omitempty"`
	Content     map[string]MediaTypeObject `json:"content"`
	Required    bool                       `json:"required,omitempty"`
}

// MediaTypeObject wraps schema and examples for a specific media type.
type MediaTypeObject struct {
	Schema   *Schema             `json:"schema,omitempty"`
	Example  any                 `json:"example,omitempty"`
	Examples map[string]Example  `json:"examples,omitempty"`
	Encoding map[string]Encoding `json:"encoding,omitempty"`
}

// Response captures the structure of an HTTP response.
type Response struct {
	Description string                     `json:"description"`
	Headers     map[string]Header          `json:"headers,omitempty"`
	Content     map[string]MediaTypeObject `json:"content,omitempty"`
	Links       map[string]Link            `json:"links,omitempty"`
}

// Schema represents an OpenAPI schema definition.
type Schema struct {
	Type                 any                `json:"type,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	AdditionalProperties any                `json:"additionalProperties,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Description          string             `json:"description,omitempty"`

	Format           string   `json:"format,omitempty"`
	Pattern          string   `json:"pattern,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MinLength        *int     `json:"minLength,omitempty"`
	MaxLength        *int     `json:"maxLength,omitempty"`
	MinItems         *int     `json:"minItems,omitempty"`
	MaxItems         *int     `json:"maxItems,omitempty"`
	UniqueItems      *bool    `json:"uniqueItems,omitempty"`
	Enum             []any    `json:"enum,omitempty"`
	Const            any      `json:"const,omitempty"`
	Default          any      `json:"default,omitempty"`
	Example          any      `json:"example,omitempty"`
	Examples         []any    `json:"examples,omitempty"`

	OneOf []*Schema `json:"oneOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	AllOf []*Schema `json:"allOf,omitempty"`
	Not   *Schema   `json:"not,omitempty"`

	Title         string                 `json:"title,omitempty"`
	Deprecated    *bool                  `json:"deprecated,omitempty"`
	ReadOnly      *bool                  `json:"readOnly,omitempty"`
	WriteOnly     *bool                  `json:"writeOnly,omitempty"`
	XML           *XML                   `json:"xml,omitempty"`
	ExternalDocs  *ExternalDocumentation `json:"externalDocs,omitempty"`
	Discriminator *Discriminator         `json:"discriminator,omitempty"`
}

// Components stores re-usable OpenAPI components.
type Components struct {
	Schemas         map[string]Schema         `json:"schemas,omitempty"`
	Responses       map[string]Response       `json:"responses,omitempty"`
	Parameters      map[string]Parameter      `json:"parameters,omitempty"`
	Examples        map[string]Example        `json:"examples,omitempty"`
	RequestBodies   map[string]RequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]Header         `json:"headers,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]Link           `json:"links,omitempty"`
	Callbacks       map[string]Callback       `json:"callbacks,omitempty"`
	PathItems       map[string]PathItem       `json:"pathItems,omitempty"`
}

// Example represents a concrete example payload.
type Example struct {
	Summary       string `json:"summary,omitempty"`
	Description   string `json:"description,omitempty"`
	Value         any    `json:"value,omitempty"`
	ExternalValue string `json:"externalValue,omitempty"`
}

// XML represents OpenAPI 3.1 XML metadata.
type XML struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Attribute bool   `json:"attribute,omitempty"`
	Wrapped   bool   `json:"wrapped,omitempty"`
}

// ExternalDocumentation represents OpenAPI 3.1 external documentation.
type ExternalDocumentation struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// Discriminator represents OpenAPI 3.1 discriminators for polymorphic schemas.
type Discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

// Header represents OpenAPI 3.1 header metadata.
type Header struct {
	Description     string              `json:"description,omitempty"`
	Required        bool                `json:"required,omitempty"`
	Deprecated      bool                `json:"deprecated,omitempty"`
	AllowEmptyValue bool                `json:"allowEmptyValue,omitempty"`
	Schema          *Schema             `json:"schema,omitempty"`
	Example         any                 `json:"example,omitempty"`
	Examples        map[string]*Example `json:"examples,omitempty"`
}

// Link represents OpenAPI 3.1 link objects.
type Link struct {
	OperationRef string         `json:"operationRef,omitempty"`
	OperationId  string         `json:"operationId,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty"`
	Description  string         `json:"description,omitempty"`
	Server       *Server        `json:"server,omitempty"`
}

// Callback represents OpenAPI 3.1 callback object.
type Callback map[string]*PathItem

// Encoding represents OpenAPI 3.1 encoding for request/response content.
type Encoding struct {
	ContentType   string             `json:"contentType,omitempty"`
	Headers       map[string]*Header `json:"headers,omitempty"`
	Style         string             `json:"style,omitempty"`
	Explode       *bool              `json:"explode,omitempty"`
	AllowReserved bool               `json:"allowReserved,omitempty"`
}

// Webhooks represents OpenAPI 3.1 webhooks.
type Webhooks map[string]*PathItem

// SecurityRequirement represents a security requirement.
type SecurityRequirement map[string][]string

// SecurityScheme represents a security scheme configuration.
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

// Tag represents an OpenAPI tag entry.
type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
