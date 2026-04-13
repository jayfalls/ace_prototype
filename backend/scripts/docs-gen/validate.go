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

func validateDocs(ctx context.Context, db *sql.DB, repoRoot string) error {
	rows, err := db.QueryContext(ctx, `
		SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name
	`)
	if err != nil {
		return fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	liveTables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scanning table name: %w", err)
		}
		liveTables[name] = true
	}
	rows.Close()

	docDir := filepath.Join(repoRoot, "documentation/database-design/schema")
	docTables := make(map[string]bool)

	entries, err := os.ReadDir(docDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "Documentation directory not found — run schema-doc-gen first.")
			return nil
		}
		return fmt.Errorf("reading doc dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			docTables[strings.TrimSuffix(entry.Name(), ".md")] = true
		}
	}

	var hasDrift bool

	// Missing docs
	var missing []string
	for table := range liveTables {
		if !docTables[table] {
			missing = append(missing, table)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		hasDrift = true
		fmt.Println("MISSING DOCUMENTATION:")
		for _, t := range missing {
			fmt.Printf("  - %s\n", t)
		}
	}

	// Extra docs
	var extra []string
	for table := range docTables {
		if !liveTables[table] {
			extra = append(extra, table)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		hasDrift = true
		fmt.Println("EXTRA DOCUMENTATION:")
		for _, t := range extra {
			fmt.Printf("  - %s\n", t)
		}
	}

	// Column drift
	var outdated []string
	for table := range liveTables {
		if !docTables[table] {
			continue
		}
		var liveCount int
		colRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info('%s')", table))
		if err != nil {
			continue
		}
		for colRows.Next() {
			liveCount++
		}
		colRows.Close()

		content, err := os.ReadFile(filepath.Join(docDir, table+".md"))
		if err != nil {
			continue
		}

		docCount := 0
		inColumnsSection := false
		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "## Columns" {
				inColumnsSection = true
				continue
			}
			if strings.HasPrefix(trimmed, "## ") && inColumnsSection {
				break
			}
			if inColumnsSection && strings.HasPrefix(trimmed, "| `") && !strings.Contains(line, "------") {
				docCount++
			}
		}

		if liveCount != docCount {
			hasDrift = true
			outdated = append(outdated, fmt.Sprintf("%s (live: %d, doc: %d)", table, liveCount, docCount))
		}
	}
	sort.Strings(outdated)
	if len(outdated) > 0 {
		fmt.Println("OUTDATED DOCUMENTATION:")
		for _, d := range outdated {
			fmt.Printf("  - %s\n", d)
		}
	}

	if hasDrift {
		fmt.Println("RESULT: DRIFT DETECTED")
		return fmt.Errorf("documentation drift detected")
	}

	fmt.Println("RESULT: ALL DOCUMENTATION UP TO DATE")
	return nil
}
