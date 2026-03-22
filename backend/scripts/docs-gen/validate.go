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

func validateDocs(ctx context.Context, conn *pgx.Conn, repoRoot string) error {
	rows, err := conn.Query(ctx, `
		SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename
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
		if err := conn.QueryRow(ctx, `
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
		`, table).Scan(&liveCount); err != nil {
			continue
		}

		content, err := os.ReadFile(filepath.Join(docDir, table+".md"))
		if err != nil {
			continue
		}

		docCount := 0
		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "| `") && !strings.Contains(line, "Name | Type") && !strings.Contains(line, "------") {
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
