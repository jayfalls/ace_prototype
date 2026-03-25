package annot8

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	typeIndex     *TypeIndex
	typeIndexOnce sync.Once
	modulePath    string // loaded from go.mod to identify internal packages
)

// Find a way to add method that will add external known types to the type index
// This is useful for types that are not defined in the current package but are known to the OpenAPI spec,
// such as types from external libraries or standard library types that we want to document.
func ensureTypeIndex() {
	// debug.PrintStack()
	typeIndexOnce.Do(func() {
		// load module path for package classification
		loadModulePath()
		slog.Debug("[annot8] cache.go: initializing typeIndex and externalKnownTypes")
		// Build type index once at startup
		typeIndex = BuildTypeIndex()

		slog.Debug("[annot8] cache.go: typeIndex built, setting externalKnownTypes")
		typeIndex.externalKnownTypes = defaultExternalKnownTypes()
		// Log the number of types and files indexed
		slog.Debug(
			"[annot8] cache.go: typeIndex initialized",
			"types",
			len(typeIndex.types),
			"files",
			len(typeIndex.files),
		)
	})
}

// TypeIndex provides fast lookup of type definitions by package and type name.
type TypeIndex struct {
	types              map[string]map[string]*ast.TypeSpec // package -> type -> spec
	files              map[string]*ast.File                // file path -> parsed file
	externalKnownTypes map[string]*Schema                  // external known types
	qualifiedTypes     map[string]*ast.TypeSpec            // qualified type name -> spec (e.g., "order.CreateReq")
	packageImports     map[string]string                   // import path -> package name (e.g., "github.com/user/sqlc" -> "sqlc")
}

// BuildTypeIndex scans the given roots and builds a type index for all Go types.
func BuildTypeIndex() *TypeIndex {
	idx := &TypeIndex{
		types:              make(map[string]map[string]*ast.TypeSpec),
		files:              make(map[string]*ast.File),
		externalKnownTypes: make(map[string]*Schema),
		qualifiedTypes:     make(map[string]*ast.TypeSpec),
		packageImports:     make(map[string]string),
	}

	// Find project root by looking for go.mod
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		slog.Debug("[annot8] BuildTypeIndex: could not find project root, using current directory")
		projectRoot = "."
	} else {
		slog.Debug("[annot8] BuildTypeIndex: using project root", "root", projectRoot)
	}

	_ = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil ||
			info.IsDir() ||
			!strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}

		return idx.indexFile(path)
	})

	idx.externalKnownTypes = defaultExternalKnownTypes()

	slog.Debug("[annot8] BuildTypeIndex: completed", "totalPackages", len(idx.types), "totalFiles", len(idx.files))
	return idx
}

func defaultExternalKnownTypes() map[string]*Schema {
	return map[string]*Schema{
		// JSON and raw data types
		"any":             {Description: "Any type (interface{})"},
		"json.RawMessage": {Description: "Raw JSON data"},
		"jsontext.Value":  {Description: "Raw JSON data"},
		"byte":            {Type: "integer", Format: "int32", Description: "Byte value"},
		"[]byte":          {Type: "string", Format: "byte", Description: "Binary data (base64-encoded)"},
		"rune":            {Type: "integer", Format: "int32", Description: "Rune (Unicode code point) value"},
		"[]rune":          {Type: "string", Description: "String data"},

		// PostgreSQL types (jackc/pgtype)
		"pgtype.Text":        {Type: "string", Description: "PostgreSQL text type"},
		"pgtype.Bool":        {Type: "boolean", Description: "PostgreSQL boolean type"},
		"pgtype.Int2":        {Type: "integer", Format: "int32", Description: "PostgreSQL smallint (int16)"},
		"pgtype.Int4":        {Type: "integer", Format: "int32", Description: "PostgreSQL integer (int32)"},
		"pgtype.Int8":        {Type: "integer", Format: "int64", Description: "PostgreSQL bigint (int64)"},
		"pgtype.Float4":      {Type: "number", Format: "float", Description: "PostgreSQL real (float32)"},
		"pgtype.Float8":      {Type: "number", Format: "double", Description: "PostgreSQL double precision (float64)"},
		"pgtype.Numeric":     {Type: "number", Description: "PostgreSQL numeric/decimal type"},
		"pgtype.Interval":    {Type: "string", Description: "PostgreSQL interval type"},
		"pgtype.Timestamptz": {Type: "string", Format: "date-time", Description: "PostgreSQL timestamp with timezone"},
		"pgtype.Timestamp": {
			Type:        "string",
			Format:      "date-time",
			Description: "PostgreSQL timestamp without timezone",
		},
		"pgtype.Date":  {Type: "string", Format: "date", Description: "PostgreSQL date type"},
		"pgtype.Point": {Type: "string", Description: "PostgreSQL point type (e.g., '(1.0,2.0)')"},
		"pgtype.UUID":  {Type: "string", Format: "uuid", Description: "PostgreSQL UUID type"},
		"pgtype.JSONB": {Description: "PostgreSQL JSONB type"},
		"pgtype.JSON":  {Description: "PostgreSQL JSON type"},

		// Time types
		"time.Time": {Type: "string", Format: "date-time", Description: "RFC3339 date-time"},
		"*time.Time": {
			Type:        []any{"string", "null"},
			Format:      "date-time",
			Description: "Nullable RFC3339 date-time",
		},
		"time.Duration": {
			Type:        "string",
			Description: "Duration string (e.g., '1h30m'). Note: default Go JSON marshal is nanoseconds (integer).",
		},
		"time.Weekday": {Type: "integer", Description: "Go time.Weekday (0=Sunday, ...)"},

		// UUID types
		"uuid.UUID": {Type: "string", Format: "uuid", Description: "UUID string"},
		"*uuid.UUID": {
			Type:        []any{"string", "null"},
			Format:      "uuid",
			Description: "Nullable UUID string",
		},

		// Network types
		"net.IP":    {Type: "string", Format: "ipv4", Description: "IPv4 address"},
		"net.IPNet": {Type: "string", Description: "IP network (CIDR notation)"},
		"url.URL":   {Type: "string", Format: "uri", Description: "URL string"},
		"*url.URL": {
			Type:        []any{"string", "null"},
			Format:      "uri",
			Description: "Nullable URL string",
		},

		// Database driver types (database/sql)
		"sql.NullString":  {Type: []any{"string", "null"}, Description: "Nullable string"},
		"sql.NullInt64":   {Type: []any{"integer", "null"}, Format: "int64", Description: "Nullable integer"},
		"sql.NullInt32":   {Type: []any{"integer", "null"}, Format: "int32", Description: "Nullable integer"},
		"sql.NullFloat64": {Type: []any{"number", "null"}, Description: "Nullable number"},
		"sql.NullBool":    {Type: []any{"boolean", "null"}, Description: "Nullable boolean"},
		"sql.NullTime":    {Type: []any{"string", "null"}, Format: "date-time", Description: "Nullable date-time"},
		"sql.RawBytes":    {Type: "string", Format: "byte", Description: "Raw database bytes (base64)"},

		// Common Go types
		"big.Int": {Type: "string", Description: "Big integer as string"},
		"*big.Int": {
			Type:        []any{"string", "null"},
			Description: "Nullable big integer as string",
		},
		"decimal.Decimal": {Type: "string", Description: "Decimal number as string"},
		"*decimal.Decimal": {
			Type:        []any{"string", "null"},
			Description: "Nullable decimal number as string",
		},
	}
}

// indexFile processes a single Go file and indexes its types
func (idx *TypeIndex) indexFile(filePath string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		slog.Debug("[annot8] BuildTypeIndex: failed to parse file", "path", filePath, "err", err)
		return nil // Continue with other files
	}

	// Normalize path for consistent lookups across platforms
	normalizedPath := filepath.ToSlash(filePath)
	idx.files[normalizedPath] = file
	pkg := file.Name.Name

	// Record package imports for external vs internal classification
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil && imp.Name.Name != "" {
			alias = imp.Name.Name
		} else {
			alias = path.Base(importPath)
		}
		idx.packageImports[importPath] = alias
	}

	if _, ok := idx.types[pkg]; !ok {
		idx.types[pkg] = make(map[string]*ast.TypeSpec)
	}

	// Index type declarations
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, spec := range gd.Specs {
				if ts, isTypeSpec := spec.(*ast.TypeSpec); isTypeSpec {
					typeName := ts.Name.Name
					qualifiedName := idx.getQualifiedTypeName(pkg, typeName)

					// Store in both maps
					idx.types[pkg][typeName] = ts
					idx.qualifiedTypes[qualifiedName] = ts

					slog.Debug(
						"[annot8] BuildTypeIndex: indexed type",
						"package", pkg,
						"type", typeName,
						"qualified", qualifiedName,
						"file", filePath,
					)
				}
			}
		}
	}

	return nil
}

func GetTypeIndex() *TypeIndex {
	if typeIndex == nil {
		slog.Error("[annot8] GetTypeIndex: typeIndex is nil, building type index")
		typeIndex = BuildTypeIndex()
	} else {
		slog.Debug("[annot8] GetTypeIndex: returning existing typeIndex")
	}
	return typeIndex
}

// LookupType returns the TypeSpec for a given package and type name, or nil if not found.
func (idx *TypeIndex) LookupType(pkg, typeName string) *ast.TypeSpec {
	if idx == nil {
		return nil
	}
	if pkgTypes, ok := idx.types[pkg]; ok {
		return pkgTypes[typeName]
	}
	return nil
}

// LookupQualifiedType returns the TypeSpec for a qualified type name (e.g., "order.CreateReq")
func (idx *TypeIndex) LookupQualifiedType(qualifiedName string) *ast.TypeSpec {
	if idx == nil {
		return nil
	}
	return idx.qualifiedTypes[qualifiedName]
}

// LookupFile returns the AST for a given file path, handling normalization and case-insensitivity on Windows.
func (idx *TypeIndex) LookupFile(filePath string) *ast.File {
	if idx == nil {
		return nil
	}
	normalized := filepath.ToSlash(filePath)
	if f, ok := idx.files[normalized]; ok {
		return f
	}

	// Case-insensitive fallback for Windows
	for p, f := range idx.files {
		if strings.EqualFold(p, normalized) {
			return f
		}
	}
	return nil
}

// LookupUnqualifiedType searches for a type across all packages and returns the first match along with qualified name
func (idx *TypeIndex) LookupUnqualifiedType(typeName string) (*ast.TypeSpec, string) {
	if idx == nil {
		return nil, ""
	}

	// First check if it's a basic type
	if isBasicType(typeName) {
		return nil, ""
	}

	// Collect candidate packages that define this type. Map iteration order
	// is nondeterministic, so we gather and sort package names to be stable.
	var candidates []string
	for pkgName, pkgTypes := range idx.types {
		if _, exists := pkgTypes[typeName]; exists {
			candidates = append(candidates, pkgName)
		}
	}
	if len(candidates) == 0 {
		return nil, ""
	}

	sort.Strings(candidates)

	// Prefer internal (non-external) packages over external ones.
	for _, pkgName := range candidates {
		if !idx.isExternalPackage(pkgName) {
			typeSpec := idx.types[pkgName][typeName]
			qualifiedName := idx.getQualifiedTypeName(pkgName, typeName)
			return typeSpec, qualifiedName
		}
	}

	// No internal candidate found; return the first external candidate (deterministic due to sorting)
	pkgName := candidates[0]
	typeSpec := idx.types[pkgName][typeName]
	qualifiedName := idx.getQualifiedTypeName(pkgName, typeName)
	return typeSpec, qualifiedName
}

// GetQualifiedTypeName returns the appropriate qualified name for a type
func (idx *TypeIndex) GetQualifiedTypeName(typeName string) string {
	// If already qualified, return as-is
	if strings.Contains(typeName, ".") {
		return typeName
	}

	// Look up the type and return its qualified name
	if _, qualifiedName := idx.LookupUnqualifiedType(typeName); qualifiedName != "" {
		return qualifiedName
	}

	// Fallback to original name
	return typeName
}

func AddExternalKnownType(name string, schema *Schema) {
	ensureTypeIndex() // Ensure typeIndex is initialized
	if typeIndex == nil {
		slog.Error("[annot8] AddExternalKnownType: typeIndex is nil, cannot add external type", "name", name)
		return
	}
	if typeIndex.externalKnownTypes == nil {
		typeIndex.externalKnownTypes = make(map[string]*Schema)
	}
	typeIndex.externalKnownTypes[name] = schema
	slog.Debug("[annot8] AddExternalKnownType: added external known type", "name", name)
}

// resetTypeIndexForTesting resets the type index for testing purposes
// This should only be used in tests
func resetTypeIndexForTesting() {
	typeIndex = nil
	typeIndexOnce = sync.Once{}
}

// getQualifiedTypeName creates a qualified type name for indexing.
// For external packages (like sqlc, pgtype), use the package name as-is.
// For internal project types, use package.TypeName format.
func (idx *TypeIndex) getQualifiedTypeName(pkg, typeName string) string {
	// Check if this is an external/third-party package
	if idx.isExternalPackage(pkg) {
		return pkg + "." + typeName
	}

	// For internal project types, use package.TypeName format
	return pkg + "." + typeName
}

// isExternalPackage determines if a package is external/third-party
func (idx *TypeIndex) isExternalPackage(pkg string) bool {
	// If an import alias maps to a path outside the current module, treat as external
	for importPath, alias := range idx.packageImports {
		if alias == pkg {
			if modulePath != "" && strings.HasPrefix(importPath, modulePath) {
				return false
			}
			return true
		}
	}
	// Default to internal
	return false
}

// findProjectRoot finds the project root by looking for go.mod file
func findProjectRoot() string {
	// Start from current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Walk up the directory tree looking for go.mod
	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return currentDir
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root
			break
		}
		currentDir = parentDir
	}

	return ""
}

// loadModulePath reads the Go module path from go.mod to distinguish internal vs external packages
func loadModulePath() {
	if modulePath != "" {
		return
	}
	root := findProjectRoot()
	if root == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			return
		}
	}
}
