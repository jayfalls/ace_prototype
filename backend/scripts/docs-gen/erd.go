package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type foreignKey struct {
	SourceTable    string
	SourceColumn   string
	TargetTable    string
	TargetColumn   string
	ConstraintName string
}

func generateERD(ctx context.Context, db *sql.DB, repoRoot string) error {
	// Get all tables
	tableRows, err := db.QueryContext(ctx, `
		SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name
	`)
	if err != nil {
		return fmt.Errorf("querying tables: %w", err)
	}
	defer tableRows.Close()

	var fks []foreignKey
	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			return fmt.Errorf("scanning table name: %w", err)
		}

		// Get foreign keys for this table
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
			fks = append(fks, foreignKey{
				ConstraintName: fmt.Sprintf("fk_%s_%d", tableName, id),
				SourceTable:    tableName,
				SourceColumn:   from,
				TargetTable:    fkTable,
				TargetColumn:   to,
			})
		}
		fkRows.Close()
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("erDiagram\n")

	for _, fk := range fks {
		sb.WriteString(fmt.Sprintf("    %s ||--o{ %s : \"%s\"\n", fk.TargetTable, fk.SourceTable, fk.SourceColumn))
	}

	if len(fks) == 0 {
		sb.WriteString("    %% No foreign key relationships found\n")
	}
	sb.WriteString("```\n")

	outputDir := filepath.Join(repoRoot, "documentation/database-design/erd")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	filePath := filepath.Join(outputDir, "master.md")
	content := "# Entity Relationship Diagram\n\n" + sb.String()
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}

	fmt.Printf("Generated: %s (%d relationship(s))\n", filePath, len(fks))
	return nil
}
