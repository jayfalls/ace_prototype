package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

type route struct {
	Method string
	Path   string
	Tag    string
}

type openAPISpec struct {
	OpenAPI string                                       `yaml:"openapi"`
	Info    openAPIInfo                                  `yaml:"info"`
	Servers []openAPIServer                              `yaml:"servers"`
	Paths   map[string]map[string]map[string]interface{} `yaml:"paths"`
	Comps   map[string]interface{}                       `yaml:"components,omitempty"`
}

type openAPIInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

type openAPIServer struct {
	URL string `yaml:"url"`
}

func generateOpenAPI(ctx context.Context, conn *pgx.Conn, repoRoot string) error {
	tables, err := discoverTables(ctx, conn)
	if err != nil {
		return fmt.Errorf("discovering tables: %w", err)
	}

	routes, err := discoverRoutes(repoRoot)
	if err != nil {
		return fmt.Errorf("discovering routes: %w", err)
	}

	spec := buildSpec(routes, tables)

	out, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling spec: %w", err)
	}

	outputDir := filepath.Join(repoRoot, "documentation/api")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	filePath := filepath.Join(outputDir, "openapi.yaml")
	if err := os.WriteFile(filePath, []byte("# Auto-generated — do not edit manually\n# Run 'make test' to regenerate\n\n"+string(out)), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}

	fmt.Printf("Generated: %s (%d paths, %d tables)\n", filePath, len(routes), len(tables))
	return nil
}

func discoverTables(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	rows, err := conn.Query(ctx, `
		SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

// discoverRoutes parses main.go and handler files to extract Chi route definitions.
func discoverRoutes(repoRoot string) ([]route, error) {
	mainPath := filepath.Join(repoRoot, "backend/services/api/cmd/main.go")
	handlerDir := filepath.Join(repoRoot, "backend/services/api/internal/handler")

	var routes []route

	// Parse main.go for route registrations
	mainRoutes, err := parseMainRoutes(mainPath)
	if err != nil {
		return nil, fmt.Errorf("parsing main.go: %w", err)
	}
	routes = append(routes, mainRoutes...)

	// Parse handler files for doc comments describing endpoints
	handlerDocs, err := parseHandlerDocs(handlerDir)
	if err == nil {
		// Enrich routes with tags from handler docs
		for i := range routes {
			for path, tag := range handlerDocs {
				if strings.Contains(routes[i].Path, path) {
					routes[i].Tag = tag
				}
			}
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Path < routes[j].Path
	})

	return routes, nil
}

// parseMainRoutes extracts route registrations like r.Get("/path", handler) from main.go.
func parseMainRoutes(path string) ([]route, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var routes []route

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Match r.Get(...), r.Post(...), r.Put(...), r.Delete(...), r.Route(...)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		method := sel.Sel.Name
		if method != "Get" && method != "Post" && method != "Put" && method != "Delete" {
			return true
		}

		if len(call.Args) < 1 {
			return true
		}

		// Extract path string from first argument
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}

		pathStr := strings.Trim(lit.Value, `"`)
		routes = append(routes, route{
			Method: strings.ToUpper(method),
			Path:   pathStr,
		})

		return true
	})

	return routes, nil
}

// parseHandlerDocs reads handler Go files to find doc comments describing endpoint purpose.
func parseHandlerDocs(dir string) (map[string]string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	docs := make(map[string]string)
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, cg := range f.Comments {
				text := cg.Text()
				// Look for endpoint path references in comments
				if strings.Contains(text, "/health") {
					docs["/health"] = "health"
				}
				if strings.Contains(text, "/examples") {
					docs["/examples"] = "examples"
				}
			}
		}
	}
	return docs, nil
}

func buildSpec(routes []route, tables []string) openAPISpec {
	paths := make(map[string]map[string]map[string]interface{})

	for _, r := range routes {
		if paths[r.Path] == nil {
			paths[r.Path] = make(map[string]map[string]interface{})
		}

		method := strings.ToLower(r.Method)
		op := map[string]interface{}{
			"summary":   formatSummary(r),
			"responses": buildResponses(r),
		}
		if r.Tag != "" {
			op["tags"] = []string{r.Tag}
		}
		// Add security to non-health endpoints
		if r.Tag != "health" && r.Tag != "observability" {
			op["security"] = []map[string][]string{{"BearerAuth": {}}}
		}

		paths[r.Path][method] = op
	}

	return openAPISpec{
		OpenAPI: "3.1.0",
		Info: openAPIInfo{
			Title:   "ACE API",
			Version: "0.1.0",
		},
		Servers: []openAPIServer{{URL: "http://localhost:8080"}},
		Paths:   paths,
		Comps: map[string]interface{}{
			"schemas": map[string]interface{}{
				"APIResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{"type": "boolean"},
						"data":    map[string]interface{}{},
					},
				},
				"APIError": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{"type": "boolean", "enum": []bool{false}},
						"error": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"code":    map[string]interface{}{"type": "string"},
								"message": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			"securitySchemes": map[string]interface{}{
				"BearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
	}
}

func formatSummary(r route) string {
	// Derive a human-readable summary from the path
	parts := strings.Split(strings.Trim(r.Path, "/"), "/")
	var words []string
	for _, p := range parts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			continue
		}
		words = append(words, p)
	}
	if len(words) == 0 {
		return r.Method + " root"
	}
	return r.Method + " " + strings.Join(words, " ")
}

func buildResponses(r route) map[string]interface{} {
	resps := map[string]interface{}{
		"200": map[string]interface{}{"description": "Success"},
	}
	if r.Method == "POST" {
		resps["201"] = map[string]interface{}{"description": "Created"}
		resps["400"] = map[string]interface{}{"description": "Validation error"}
	}
	if strings.Contains(r.Path, "/health/ready") {
		resps["503"] = map[string]interface{}{"description": "Not ready"}
	}
	return resps
}
