// Package plugin is the v0.9 extension runtime (spec 9.1): manifests declare
// what a plugin provides, depends on, may egress to and needs as secrets —
// parsed without running plugin code.
package plugin

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manifest is a plugin.toml declaration. Entry is a built-in id or
// "mcp:<command>" — the registry maps kinds to ports.
type Manifest struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Kind     string   `json:"kind"`     // tool | channel | gate | sense | context
	Provides []string `json:"provides"` // capability keys this plugin owns
	Depends  []string `json:"depends"`  // capability keys it may look up
	Egress   []string `json:"egress"`   // host allowlist for outbound calls
	Secrets  []string `json:"secrets"`  // env names brokered at call time
	Entry    string   `json:"entry"`    // built-in id or mcp:<command>
	Dir      string   `json:"-"`        // manifest directory (tool-relative resources)
}

// ParseManifest reads plugin.toml without executing anything. The format is
// the flat TOML subset used by plugin.toml: `key = "v"` | `key = ["a","b"]`
// with `#` comments — no tables, so a stdlib parser suffices.
func ParseManifest(path string) (Manifest, error) {
	var m Manifest
	f, err := os.Open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return m, fmt.Errorf("%s:%d: expected key = value", path, lineNo)
		}
		key, val := strings.TrimSpace(k), strings.TrimSpace(v)
		strs, err := parseValue(val)
		if err != nil {
			return m, fmt.Errorf("%s:%d: %v", path, lineNo, err)
		}
		switch key {
		case "name":
			m.Name = first(strs)
		case "version":
			m.Version = first(strs)
		case "kind":
			m.Kind = first(strs)
		case "provides":
			m.Provides = strs
		case "depends":
			m.Depends = strs
		case "egress":
			m.Egress = strs
		case "secrets":
			m.Secrets = strs
		case "entry":
			m.Entry = first(strs)
		default:
			return m, fmt.Errorf("%s:%d: unknown key %q", path, lineNo, key)
		}
	}
	if err := sc.Err(); err != nil {
		return m, err
	}
	if m.Name == "" || m.Kind == "" {
		return m, fmt.Errorf("%s: name and kind are required", path)
	}
	m.Dir = filepath.Dir(path)
	return m, nil
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// parseValue accepts "str" or ["a", "b", ...] — both forms yield []string so
// scalar fields and list fields share one code path.
func parseValue(v string) ([]string, error) {
	if strings.HasPrefix(v, "[") {
		if !strings.HasSuffix(v, "]") {
			return nil, fmt.Errorf("unterminated array")
		}
		inner := strings.TrimSpace(v[1 : len(v)-1])
		if inner == "" {
			return nil, nil
		}
		var out []string
		for _, part := range strings.Split(inner, ",") {
			part = strings.TrimSpace(part)
			s, err := unquote(part)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	}
	s, err := unquote(v)
	if err != nil {
		return nil, err
	}
	return []string{s}, nil
}

func unquote(s string) (string, error) {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return strings.ReplaceAll(s[1:len(s)-1], `\"`, `"`), nil
	}
	return "", fmt.Errorf("value must be quoted: %q", s)
}
