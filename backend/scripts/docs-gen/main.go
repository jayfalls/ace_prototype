package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
)

func main() {
	fmt.Println("=== Documentation Generation Pipeline ===")
	fmt.Println()

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not find repo root: %v\n", err)
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/ace?sslmode=disable"
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to database: %v\n", err)
		fmt.Fprintln(os.Stderr, "Hint: Set DATABASE_URL environment variable or ensure PostgreSQL is running.")
		os.Exit(1)
	}
	defer conn.Close(ctx)

	fmt.Println("--- Schema Documentation ---")
	if err := generateSchemaDocs(ctx, conn, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schema docs: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	fmt.Println("--- ERD Generation ---")
	if err := generateERD(ctx, conn, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating ERD: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	fmt.Println("--- Documentation Validation ---")
	if err := validateDocs(ctx, conn, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error validating docs: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	fmt.Println("--- OpenAPI Generation ---")
	if err := generateOpenAPI(ctx, conn, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating OpenAPI spec: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

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
