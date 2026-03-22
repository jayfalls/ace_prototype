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

type column struct {
	Name          string
	DataType      string
	ColumnDefault string
	IsNullable    string
}

type constraint struct {
	Name       string
	Type       string
	Definition string
}

type index struct {
	Name       string
	Definition string
	IsUnique   bool
}

type tableDoc struct {
	Schema      string
	Name        string
	Columns     []column
	Constraints []constraint
	Indexes     []index
}

func generateSchemaDocs(ctx context.Context, conn *pgx.Conn, repoRoot string) error {
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
		return fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	tables := make(map[string]*tableDoc)
	var tableOrder []string

	for rows.Next() {
		var schema, tableName, colName, dataType, colDefault, isNullable string
		if err := rows.Scan(&schema, &tableName, &colName, &dataType, &colDefault, &isNullable); err != nil {
			return fmt.Errorf("scanning column: %w", err)
		}
		if _, exists := tables[tableName]; !exists {
			tables[tableName] = &tableDoc{Schema: schema, Name: tableName}
			tableOrder = append(tableOrder, tableName)
		}
		tables[tableName].Columns = append(tables[tableName].Columns, column{
			Name: colName, DataType: dataType, ColumnDefault: colDefault, IsNullable: isNullable,
		})
	}
	rows.Close()

	constraintRows, err := conn.Query(ctx, `
		SELECT
			tc.table_name,
			tc.constraint_name,
			tc.constraint_type,
			COALESCE(pg_get_constraintdef(pgc.oid), '')
		FROM information_schema.table_constraints tc
		JOIN pg_constraint pgc ON pgc.conname = tc.constraint_name
		JOIN pg_namespace nsp ON nsp.oid = pgc.connamespace
		WHERE tc.table_schema = 'public' AND nsp.nspname = 'public'
		ORDER BY tc.table_name, tc.constraint_name
	`)
	if err != nil {
		return fmt.Errorf("querying constraints: %w", err)
	}
	defer constraintRows.Close()

	for constraintRows.Next() {
		var tableName, constraintName, constraintType, definition string
		if err := constraintRows.Scan(&tableName, &constraintName, &constraintType, &definition); err != nil {
			return fmt.Errorf("scanning constraint: %w", err)
		}
		if t, exists := tables[tableName]; exists {
			t.Constraints = append(t.Constraints, constraint{Name: constraintName, Type: constraintType, Definition: definition})
		}
	}
	constraintRows.Close()

	indexRows, err := conn.Query(ctx, `
		SELECT t.relname, i.relname, pg_get_indexdef(ix.indexrelid), ix.indisunique
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'public' AND NOT ix.indisprimary
		ORDER BY t.relname, i.relname
	`)
	if err != nil {
		return fmt.Errorf("querying indexes: %w", err)
	}
	defer indexRows.Close()

	for indexRows.Next() {
		var tableName, indexName, indexDef string
		var isUnique bool
		if err := indexRows.Scan(&tableName, &indexName, &indexDef, &isUnique); err != nil {
			return fmt.Errorf("scanning index: %w", err)
		}
		if t, exists := tables[tableName]; exists {
			t.Indexes = append(t.Indexes, index{Name: indexName, Definition: indexDef, IsUnique: isUnique})
		}
	}
	indexRows.Close()

	outputDir := filepath.Join(repoRoot, "documentation/database-design/schema")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	sort.Strings(tableOrder)
	for _, tableName := range tableOrder {
		content := formatTableMarkdown(tables[tableName])
		filePath := filepath.Join(outputDir, tableName+".md")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filePath, err)
		}
		fmt.Printf("Generated: %s\n", filePath)
	}

	fmt.Printf("Schema documentation generated for %d table(s).\n", len(tables))
	return nil
}

func formatTableMarkdown(doc *tableDoc) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", doc.Name))
	sb.WriteString(fmt.Sprintf("Schema: `%s`\n\n", doc.Schema))

	sb.WriteString("## Columns\n\n")
	sb.WriteString("| Column | Type | Default | Nullable |\n")
	sb.WriteString("|--------|------|---------|----------|\n")
	for _, col := range doc.Columns {
		d := col.ColumnDefault
		if d == "" {
			d = "-"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", col.Name, col.DataType, d, col.IsNullable))
	}
	sb.WriteString("\n")

	if len(doc.Constraints) > 0 {
		sb.WriteString("## Constraints\n\n")
		sb.WriteString("| Name | Type | Definition |\n")
		sb.WriteString("|------|------|------------|\n")
		for _, c := range doc.Constraints {
			sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` |\n", c.Name, c.Type, c.Definition))
		}
		sb.WriteString("\n")
	}

	if len(doc.Indexes) > 0 {
		sb.WriteString("## Indexes\n\n")
		sb.WriteString("| Name | Unique | Definition |\n")
		sb.WriteString("|------|--------|------------|\n")
		for _, idx := range doc.Indexes {
			u := "No"
			if idx.IsUnique {
				u = "Yes"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` |\n", idx.Name, u, idx.Definition))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
