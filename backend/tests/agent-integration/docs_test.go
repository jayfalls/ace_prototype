package agentintegration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLoadAPIDocs verifies that the OpenAPI spec exists and is valid YAML.
// FSD Requirement: FR-6.5
func TestLoadAPIDocs(t *testing.T) {
	data, err := os.ReadFile("../../documentation/api/openapi.yaml")
	if err != nil {
		t.Fatalf("openapi.yaml not found: %v", err)
	}

	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("openapi.yaml is not valid YAML: %v", err)
	}

	openapiVersion, ok := spec["openapi"].(string)
	if !ok {
		t.Fatal("openapi.yaml missing 'openapi' field")
	}

	if !strings.HasPrefix(openapiVersion, "3.1") {
		t.Errorf("expected OpenAPI 3.1.x, got %s", openapiVersion)
	}

	// Verify paths section exists
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("openapi.yaml missing 'paths' section")
	}

	if len(paths) == 0 {
		t.Error("openapi.yaml has no paths defined")
	}

	// Verify expected endpoints exist
	expectedPaths := []string{"/", "/health/live", "/health/ready", "/health/exporters", "/metrics", "/examples/", "/examples/{id}"}
	for _, p := range expectedPaths {
		if _, exists := paths[p]; !exists {
			t.Errorf("openapi.yaml missing expected path: %s", p)
		}
	}
}

// TestSchemaDocsExist verifies that schema documentation directories contain markdown files.
// FSD Requirement: FR-1.1
func TestSchemaDocsExist(t *testing.T) {
	// Currently only the usage entity group has a table
	dirs := []string{
		"../../documentation/database-design/schema/usage",
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Errorf("directory %s not found: %v", dir, err)
			continue
		}

		hasMarkdown := false
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".md" {
				hasMarkdown = true
				break
			}
		}
		if !hasMarkdown {
			t.Errorf("directory %s contains no markdown files", dir)
		}
	}
}

// TestNamingConventions verifies that the naming conventions document exists
// and contains the key rules.
// FSD Requirement: FR-3.1
func TestNamingConventions(t *testing.T) {
	data, err := os.ReadFile("../../documentation/database-design/conventions.md")
	if err != nil {
		t.Fatalf("conventions.md not found: %v", err)
	}

	content := string(data)
	requiredPatterns := []string{
		"snake_case",
		"idx_{table}_{columns}",
		"TIMESTAMPTZ",
		"gen_random_uuid()",
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(content, pattern) {
			t.Errorf("conventions.md missing required pattern: %s", pattern)
		}
	}
}

// TestAgentDocsExist verifies that all agent-facing documentation files exist.
// FSD Requirement: FR-6.1, FR-6.2, FR-6.3, FR-6.5
func TestAgentDocsExist(t *testing.T) {
	requiredFiles := []string{
		"../../documentation/agents/api-reference.md",
		"../../documentation/agents/schema-generation.md",
		"../../documentation/agents/patterns.md",
		"../../documentation/agents/config-updates.md",
		"../../documentation/agents/training/adding-a-table.md",
	}

	for _, f := range requiredFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("required agent documentation file missing: %s", f)
		}
	}
}
