package ace

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"ace/internal/science"

	_ "modernc.org/sqlite"
)

// renderConceptRegistry produces the canonical docs/CONCEPT_REGISTRY.md from
// the seeded registry so the ledger is generated, never hand-edited.
func renderConceptRegistry(t *testing.T) string {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := science.Migrate(db); err != nil {
		t.Fatal(err)
	}
	cs, err := science.Concepts(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("# Concept Registry\n\nGenerated from `internal/science` concept_registry seeds — do not hand-edit.\n\n")
	b.WriteString("| name | kind | owner | purpose | supersedes | superseded_by |\n|---|---|---|---|---|---|\n")
	for _, c := range cs {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", c.Name, c.Kind, c.Owner, c.Purpose, c.Supersedes, c.SupersededBy)
	}
	return b.String()
}

// TestConceptRegistryFresh fails when the committed ledger drifts from the
// seeded registry (8.8: generated ledger, same-step cleanup).
func TestConceptRegistryFresh(t *testing.T) {
	want := renderConceptRegistry(t)
	if os.Getenv("GEN_REGISTRY") == "1" {
		_ = os.WriteFile(filepath.Join("docs", "CONCEPT_REGISTRY.md"), []byte(want), 0o644)
		return
	}
	got, err := os.ReadFile(filepath.Join("docs", "CONCEPT_REGISTRY.md"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if string(got) != want {
		t.Fatal("docs/CONCEPT_REGISTRY.md is stale — regenerate it from the registry seeds")
	}
}

// TestSupersededPathsRemoved enforces same-step cleanup: a registry entry
// marked superseded_by must not still exist as a live path outside archive/.
func TestSupersededPathsRemoved(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := science.Migrate(db); err != nil {
		t.Fatal(err)
	}
	cs, _ := science.Concepts(context.Background(), db)
	for _, c := range cs {
		if c.SupersededBy == "" || strings.HasPrefix(c.Name, "archive/") {
			continue
		}
		if _, err := os.Stat(c.Name); err == nil {
			t.Errorf("superseded concept %s (by %s) still exists live", c.Name, c.SupersededBy)
		}
	}
}

var docSymbolRe = regexp.MustCompile("`([A-Z][A-Za-z0-9]+(?:\\.[A-Za-z_][A-Za-z0-9_]*)?)`")

// TestDocsNameLiveSymbols flags backticked Go-style symbols in docs/*.md
// that no longer exist in code — the stale-reference count (8.8).
func TestDocsNameLiveSymbols(t *testing.T) {
	var code strings.Builder
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "archive", "docs", "web", "testdata":
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".ace") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			data, _ := os.ReadFile(path)
			code.Write(data)
			code.WriteByte('\n')
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	src := code.String()
	allow := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join("testdata", "doc_symbols_allowlist.txt")); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if s := strings.TrimSpace(l); s != "" && !strings.HasPrefix(s, "#") {
				allow[s] = true
			}
		}
	}
	stale := 0
	_ = filepath.WalkDir("docs", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, _ := os.ReadFile(path)
		for _, m := range docSymbolRe.FindAllStringSubmatch(string(data), -1) {
			sym := m[1]
			if strings.HasSuffix(sym, ".md") || strings.HasSuffix(sym, ".go") || strings.HasSuffix(sym, ".py") {
				continue // filename reference, not a symbol
			}
			if allow[sym] || strings.Contains(src, sym) || len(sym) < 4 {
				continue
			}
			// Single-hump backticked words (e.g. `Constitution`) are usually
			// prose; only flag pkg.Name or multi-hump CamelCase identifiers.
			if !strings.Contains(sym, ".") && !regexp.MustCompile(`[A-Z][a-z]+[A-Z]`).MatchString(sym) {
				continue
			}
			t.Errorf("%s names removed symbol `%s`", path, sym)
			stale++
		}
		return nil
	})
	t.Logf("stale-reference count: %d", stale)
}
