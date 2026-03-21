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

// Column represents a table column with its metadata.
type Column struct {
	Name          string
	DataType      string
	ColumnDefault string
	IsNullable    string
}

// Constraint represents a table constraint (PK, FK, UNIQUE, CHECK).
type Constraint struct {
	Name       string
	Type       string
	Definition string
}

// Index represents a table index.
type Index struct {
	Name       string
	Definition string
	IsUnique   bool
}

// TableDoc holds all documentation metadata for a table.
type TableDoc struct {
	Schema      string
	Name        string
	Columns     []Column
	Constraints []Constraint
	Indexes     []Index
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

	// Query columns
	rows, err := conn.Query(ctx, `
		SELECT 
			c.table_schema,
			c.table_name,
			c.column_name,
			c.data_type,
			COALESCE(c.column_default, ''),
			c.is_nullable
		FROM information_schema.columns c
		WHERE c.table_schema = 'public'
		ORDER BY c.table_name, c.ordinal_position
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying columns: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	tables := make(map[string]*TableDoc)
	var tableOrder []string

	for rows.Next() {
		var schema, tableName, colName, dataType, colDefault, isNullable string
		if err := rows.Scan(&schema, &tableName, &colName, &dataType, &colDefault, &isNullable); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning column row: %v\n", err)
			os.Exit(1)
		}

		if _, exists := tables[tableName]; !exists {
			tables[tableName] = &TableDoc{
				Schema: schema,
				Name:   tableName,
			}
			tableOrder = append(tableOrder, tableName)
		}

		tables[tableName].Columns = append(tables[tableName].Columns, Column{
			Name:          colName,
			DataType:      dataType,
			ColumnDefault: colDefault,
			IsNullable:    isNullable,
		})
	}
	rows.Close()

	// Query constraints
	constraintRows, err := conn.Query(ctx, `
		SELECT
			tc.table_name,
			tc.constraint_name,
			tc.constraint_type,
			COALESCE(pg_get_constraintdef(pgc.oid), '')
		FROM information_schema.table_constraints tc
		JOIN pg_constraint pgc ON pgc.conname = tc.constraint_name
		JOIN pg_namespace nsp ON nsp.oid = pgc.connamespace
		WHERE tc.table_schema = 'public'
		  AND nsp.nspname = 'public'
		ORDER BY tc.table_name, tc.constraint_name
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying constraints: %v\n", err)
		os.Exit(1)
	}
	defer constraintRows.Close()

	for constraintRows.Next() {
		var tableName, constraintName, constraintType, definition string
		if err := constraintRows.Scan(&tableName, &constraintName, &constraintType, &definition); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning constraint row: %v\n", err)
			os.Exit(1)
		}
		if t, exists := tables[tableName]; exists {
			t.Constraints = append(t.Constraints, Constraint{
				Name:       constraintName,
				Type:       constraintType,
				Definition: definition,
			})
		}
	}
	constraintRows.Close()

	// Query indexes
	indexRows, err := conn.Query(ctx, `
		SELECT
			t.relname AS table_name,
			i.relname AS index_name,
			pg_get_indexdef(ix.indexrelid) AS index_def,
			ix.indisunique AS is_unique
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'public'
		  AND NOT ix.indisprimary
		ORDER BY t.relname, i.relname
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying indexes: %v\n", err)
		os.Exit(1)
	}
	defer indexRows.Close()

	for indexRows.Next() {
		var tableName, indexName, indexDef string
		var isUnique bool
		if err := indexRows.Scan(&tableName, &indexName, &indexDef, &isUnique); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning index row: %v\n", err)
			os.Exit(1)
		}
		if t, exists := tables[tableName]; exists {
			t.Indexes = append(t.Indexes, Index{
				Name:       indexName,
				Definition: indexDef,
				IsUnique:   isUnique,
			})
		}
	}
	indexRows.Close()

	// Generate markdown files
	outputDir := "documentation/database-design/schema"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	sort.Strings(tableOrder)
	for _, tableName := range tableOrder {
		doc := tables[tableName]
		content := generateTableMarkdown(doc)
		filePath := filepath.Join(outputDir, tableName+".md")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filePath, err)
			os.Exit(1)
		}
		fmt.Printf("Generated: %s\n", filePath)
	}

	fmt.Printf("\nSchema documentation generated for %d table(s).\n", len(tables))
}

func generateTableMarkdown(doc *TableDoc) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", doc.Name))
	sb.WriteString(fmt.Sprintf("Schema: `%s`\n\n", doc.Schema))

	// Columns table
	sb.WriteString("## Columns\n\n")
	sb.WriteString("| Column | Type | Default | Nullable |\n")
	sb.WriteString("|--------|------|---------|----------|\n")
	for _, col := range doc.Columns {
		defaultVal := col.ColumnDefault
		if defaultVal == "" {
			defaultVal = "-"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", col.Name, col.DataType, defaultVal, col.IsNullable))
	}
	sb.WriteString("\n")

	// Constraints
	if len(doc.Constraints) > 0 {
		sb.WriteString("## Constraints\n\n")
		sb.WriteString("| Name | Type | Definition |\n")
		sb.WriteString("|------|------|------------|\n")
		for _, c := range doc.Constraints {
			sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` |\n", c.Name, c.Type, c.Definition))
		}
		sb.WriteString("\n")
	}

	// Indexes
	if len(doc.Indexes) > 0 {
		sb.WriteString("## Indexes\n\n")
		sb.WriteString("| Name | Unique | Definition |\n")
		sb.WriteString("|------|--------|------------|\n")
		for _, idx := range doc.Indexes {
			unique := "No"
			if idx.IsUnique {
				unique = "Yes"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` |\n", idx.Name, unique, idx.Definition))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
