package ace

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDomainStringsInProduction scans internal/ and cmd/ for hardcoded
// domain-specific identifiers from the retired v0.7 experiment harness.
// The production core is domain-agnostic: experiment names, competitor names,
// and v0.x harness markers must not appear in shipped code.
// Exempt: files under testdata/ and profiles/ directories (fixtures/personas).
func TestNoDomainStringsInProduction(t *testing.T) {
	var patterns []string
	f, err := os.Open(filepath.Join("testdata", "domain_strings.txt"))
	if err != nil {
		t.Fatalf("open domain string list: %v", err)
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r\n")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	f.Close()
	if len(patterns) == 0 {
		t.Fatal("domain_strings.txt is empty")
	}

	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "testdata" || name == "profiles" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, pat := range patterns {
				if strings.Contains(string(data), pat) {
					t.Errorf("%s contains forbidden domain string %q", path, pat)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
