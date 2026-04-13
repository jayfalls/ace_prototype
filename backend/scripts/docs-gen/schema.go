package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func generateSchemaDocs(ctx context.Context, db *sql.DB, repoRoot string) error {
	// Get all tables
	tables := make(map[string]*tableDoc)
	var tableOrder []string

	tableRows, err := db.QueryContext(ctx, `
		SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name
	`)
	if err != nil {
		return fmt.Errorf("querying tables: %w", err)
	}
	defer tableRows.Close()

	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			return fmt.Errorf("scanning table name: %w", err)
		}
		tables[tableName] = &tableDoc{Schema: "main", Name: tableName}
		tableOrder = append(tableOrder, tableName)
	}

	// Get columns for each table using PRAGMA
	for _, tableName := range tableOrder {
		colRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info('%s')", tableName))
		if err != nil {
			return fmt.Errorf("querying columns for %s: %w", tableName, err)
		}
		for colRows.Next() {
			var cid int
			var name, colType, notNull, pk string
			var defaultValue sql.NullString
			if err := colRows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
				colRows.Close()
				return fmt.Errorf("scanning column: %w", err)
			}
			nullable := "YES"
			if notNull == "1" {
				nullable = "NO"
			}
			defVal := "-"
			if defaultValue.Valid {
				defVal = defaultValue.String
			}
			tables[tableName].Columns = append(tables[tableName].Columns, column{
				Name: name, DataType: colType, ColumnDefault: defVal, IsNullable: nullable,
			})
		}
		colRows.Close()
	}

	// Get foreign keys for each table
	for _, tableName := range tableOrder {
		fkRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list('%s')", tableName))
		if err != nil {
			return fmt.Errorf("querying foreign keys for %s: %w", tableName, err)
		}
		for fkRows.Next() {
			var id, seq int
			var fkTable, from, to string
			var onUpdate, onDelete, match sql.NullString
			if err := fkRows.Scan(&id, &seq, &fkTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				fkRows.Close()
				return fmt.Errorf("scanning foreign key: %w", err)
			}
			def := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)", from, fkTable, to)
			tables[tableName].Constraints = append(tables[tableName].Constraints, constraint{
				Name: fmt.Sprintf("fk_%s_%d", tableName, id), Type: "FOREIGN KEY", Definition: def,
			})
		}
		fkRows.Close()
	}

	// Get indexes for each table
	for _, tableName := range tableOrder {
		idxRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_list('%s')", tableName))
		if err != nil {
			return fmt.Errorf("querying indexes for %s: %w", tableName, err)
		}
		for idxRows.Next() {
			var seq int
			var name, unique, origin string
			var partial int
			if err := idxRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				idxRows.Close()
				return fmt.Errorf("scanning index: %w", err)
			}
			// Skip primary key indexes
			if strings.HasPrefix(name, "sqlite_autoindex") || origin == "pk" {
				continue
			}
			isUnique := unique == "1"

			// Get index definition
			defRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_info('%s')", name))
			if err != nil {
				defRows.Close()
				return fmt.Errorf("querying index info: %w", err)
			}
			var cols []string
			for defRows.Next() {
				var cid int
				var colName string
				var colDesc string
				if err := defRows.Scan(&cid, &colName, &colDesc); err != nil {
					defRows.Close()
					return fmt.Errorf("scanning index column: %w", err)
				}
				cols = append(cols, colName)
			}
			defRows.Close()

			def := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
				map[bool]string{true: "UNIQUE ", false: ""}[isUnique], name, tableName, strings.Join(cols, ", "))
			tables[tableName].Indexes = append(tables[tableName].Indexes, index{
				Name: name, Definition: def, IsUnique: isUnique,
			})
		}
		idxRows.Close()
	}

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
