package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	fmt.Println("=== Documentation Generation Pipeline ===")
	fmt.Println()

	// Find repo root by locating .git directory
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not find repo root: %v\n", err)
		os.Exit(1)
	}

	scripts := []struct {
		name string
		dir  string
	}{
		{"Schema Documentation", "backend/scripts/schema-doc-gen"},
		{"ERD Generation", "backend/scripts/erd-gen"},
		{"Documentation Validation", "backend/scripts/validate-docs"},
		{"OpenAPI Generation", "backend/scripts/openapi-gen"},
	}

	for _, script := range scripts {
		fmt.Printf("--- %s ---\n", script.name)
		cmd := exec.Command("go", "run", ".")
		cmd.Dir = filepath.Join(repoRoot, script.dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running %s: %v\n", script.name, err)
			os.Exit(1)
		}
		fmt.Println()
	}

	fmt.Println("=== Documentation Generation Complete ===")
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find .git directory")
		}
		dir = parent
	}
}
