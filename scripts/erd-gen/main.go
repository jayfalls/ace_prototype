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

// ForeignKey represents a foreign key relationship between tables.
type ForeignKey struct {
	SourceTable    string
	SourceColumn   string
	TargetTable    string
	TargetColumn   string
	ConstraintName string
}

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

	// Query foreign key relationships
	rows, err := conn.Query(ctx, `
		SELECT
			tc.constraint_name,
			kcu.table_name AS source_table,
			kcu.column_name AS source_column,
			ccu.table_name AS target_table,
			ccu.column_name AS target_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		  AND tc.table_schema = 'public'
		ORDER BY tc.constraint_name
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying foreign keys: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.ConstraintName, &fk.SourceTable, &fk.SourceColumn, &fk.TargetTable, &fk.TargetColumn); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning foreign key row: %v\n", err)
			os.Exit(1)
		}
		fks = append(fks, fk)
	}
	rows.Close()

	// Generate Mermaid ERD
	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("erDiagram\n")

	// Collect all tables involved in relationships
	tableSet := make(map[string]bool)
	for _, fk := range fks {
		tableSet[fk.SourceTable] = true
		tableSet[fk.TargetTable] = true
	}

	// Sort tables for deterministic output
	var tables []string
	for t := range tableSet {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	// Output relationships
	for _, fk := range fks {
		sb.WriteString(fmt.Sprintf("    %s ||--o{ %s : \"%s\"\n", fk.TargetTable, fk.SourceTable, fk.SourceColumn))
	}

	// If no relationships found, note it
	if len(fks) == 0 {
		sb.WriteString("    %% No foreign key relationships found\n")
	}

	sb.WriteString("```\n")

	// Write to output file
	outputDir := "documentation/database-design/erd"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	filePath := filepath.Join(outputDir, "master.md")
	content := "# Entity Relationship Diagram\n\n" + sb.String()
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filePath, err)
		os.Exit(1)
	}

	fmt.Printf("Generated: %s\n", filePath)
	fmt.Printf("Found %d foreign key relationship(s).\n", len(fks))
}
