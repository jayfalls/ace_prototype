package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
)

type foreignKey struct {
	SourceTable    string
	SourceColumn   string
	TargetTable    string
	TargetColumn   string
	ConstraintName string
}

func generateERD(ctx context.Context, conn *pgx.Conn, repoRoot string) error {
	rows, err := conn.Query(ctx, `
		SELECT
			tc.constraint_name,
			kcu.table_name AS source_table,
			kcu.column_name AS source_column,
			ccu.table_name AS target_table,
			ccu.column_name AS target_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public'
		ORDER BY tc.constraint_name
	`)
	if err != nil {
		return fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []foreignKey
	for rows.Next() {
		var fk foreignKey
		if err := rows.Scan(&fk.ConstraintName, &fk.SourceTable, &fk.SourceColumn, &fk.TargetTable, &fk.TargetColumn); err != nil {
			return fmt.Errorf("scanning foreign key: %w", err)
		}
		fks = append(fks, fk)
	}
	rows.Close()

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
