package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
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

	// Query live tables from pg_catalog
	rows, err := conn.Query(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying tables: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	liveTables := make(map[string]bool)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning table name: %v\n", err)
			os.Exit(1)
		}
		liveTables[tableName] = true
	}
	rows.Close()

	// Query documented tables from filesystem
	docDir := "documentation/database-design/schema"
	docTables := make(map[string]bool)

	entries, err := os.ReadDir(docDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Documentation directory not found: %s\n", docDir)
			fmt.Fprintln(os.Stderr, "Run schema-doc-gen first to generate documentation.")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error reading documentation directory: %v\n", err)
		os.Exit(1)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			tableName := strings.TrimSuffix(entry.Name(), ".md")
			docTables[tableName] = true
		}
	}

	// Compare and report
	var hasDrift bool

	// Tables in live DB but missing from docs (missing docs)
	fmt.Println("=== Schema Documentation Validation ===")
	fmt.Println()

	var missingDocs []string
	for table := range liveTables {
		if !docTables[table] {
			missingDocs = append(missingDocs, table)
		}
	}
	sort.Strings(missingDocs)

	if len(missingDocs) > 0 {
		hasDrift = true
		fmt.Println("MISSING DOCUMENTATION (tables exist in DB but not documented):")
		for _, table := range missingDocs {
			fmt.Printf("  - %s\n", table)
		}
		fmt.Println()
	}

	// Tables in docs but missing from live DB (extra/stale docs)
	var extraDocs []string
	for table := range docTables {
		if !liveTables[table] {
			extraDocs = append(extraDocs, table)
		}
	}
	sort.Strings(extraDocs)

	if len(extraDocs) > 0 {
		hasDrift = true
		fmt.Println("EXTRA DOCUMENTATION (documented but table no longer exists):")
		for _, table := range extraDocs {
			fmt.Printf("  - %s\n", table)
		}
		fmt.Println()
	}

	// Check for outdated documentation by comparing column counts
	fmt.Println("CHECKING COLUMN DRIFT...")
	var outdatedDocs []string
	for table := range liveTables {
		if !docTables[table] {
			continue
		}

		// Count live columns
		var liveColumnCount int
		err := conn.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
		`, table).Scan(&liveColumnCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error counting columns for %s: %v\n", table, err)
			continue
		}

		// Count documented columns (count lines with | `column` | pattern)
		docPath := filepath.Join(docDir, table+".md")
		content, err := os.ReadFile(docPath)
		if err != nil {
			continue
		}

		docColumnCount := 0
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			// Count rows in the columns table (lines starting with | `)
			if strings.HasPrefix(strings.TrimSpace(line), "| `") && !strings.Contains(line, "Name | Type") && !strings.Contains(line, "------") {
				docColumnCount++
			}
		}

		if liveColumnCount != docColumnCount {
			hasDrift = true
			outdatedDocs = append(outdatedDocs, fmt.Sprintf("%s (live: %d cols, doc: %d cols)", table, liveColumnCount, docColumnCount))
		}
	}
	sort.Strings(outdatedDocs)

	if len(outdatedDocs) > 0 {
		fmt.Println()
		fmt.Println("OUTDATED DOCUMENTATION (column count mismatch):")
		for _, doc := range outdatedDocs {
			fmt.Printf("  - %s\n", doc)
		}
	} else {
		fmt.Println("  No column drift detected.")
	}
	fmt.Println()

	// Summary
	if hasDrift {
		fmt.Println("RESULT: DRIFT DETECTED")
		os.Exit(1)
	}

	fmt.Println("RESULT: ALL DOCUMENTATION UP TO DATE")
	os.Exit(0)
}
