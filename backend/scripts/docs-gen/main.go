package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	_ "modernc.org/sqlite"
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
		xdgDataHome := xdg.DataHome
		dbPath := filepath.Join(xdgDataHome, "ace", "ace.db")
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	}

	ctx := context.Background()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to database: %v\n", err)
		fmt.Fprintln(os.Stderr, "Hint: Set DATABASE_URL environment variable or ensure SQLite database exists.")
		os.Exit(1)
	}

	fmt.Println("--- Schema Documentation ---")
	if err := generateSchemaDocs(ctx, db, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schema docs: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	fmt.Println("--- ERD Generation ---")
	if err := generateERD(ctx, db, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating ERD: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	fmt.Println("--- Documentation Validation ---")
	if err := validateDocs(ctx, db, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error validating docs: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	fmt.Println("--- OpenAPI Generation ---")
	if err := generateOpenAPI(ctx, db, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating OpenAPI: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	fmt.Println("=== Documentation Generation Complete ===")
}

func findRepoRoot() (string, error) {
	// First, try to find .git directory by traversing up
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
			break
		}
		dir = parent
	}

	// Fallback: check for /documentation mount (container environment)
	if _, err := os.Stat("/documentation"); err == nil {
		return "/", nil
	}

	return "", fmt.Errorf("could not find repo root: no .git directory or /documentation mount found")
}
