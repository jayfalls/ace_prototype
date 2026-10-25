package specs

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateSpec enforces the living-spec size cap: at most maxBytes
// (specs.max_bytes, default 2,800 B). Over the cap the spec is rejected —
// it must be consolidated, not appended (spec 9.6).
func ValidateSpec(text string, maxBytes int) error {
	if maxBytes <= 0 {
		maxBytes = DefaultSpecMaxBytes
	}
	if len(text) > maxBytes {
		return fmt.Errorf("living spec is %d bytes, exceeds the %d-byte cap — consolidate, don't append", len(text), maxBytes)
	}
	return nil
}

// outOfScopeSectionRe finds the "out of scope" section body (## header
// until the next ## or EOF). The spec format is (in scope, out of scope,
// nuances); the heading wording is loose so we accept several forms.
var outOfScopeSectionRe = regexp.MustCompile(`(?im)^#{1,3}\s*(?:out[- ]of[- ]scope|out of scope|non[- ]?goals?|exclusions?)\s*$`)

// OutOfScopeEntries lists the path-like entries declared out of scope.
func OutOfScopeEntries(spec string) []string {
	loc := outOfScopeSectionRe.FindStringIndex(spec)
	if loc == nil {
		return nil
	}
	body := spec[loc[1]:]
	if next := strings.Index(body, "\n##"); next >= 0 {
		body = body[:next]
	}
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*•"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// CheckOutOfScope rejects output touching a path declared out of scope:
// a touched path that is (or sits under) a listed entry diverges from
// the living spec (spec 9.6: the output-vs-spec check rejects divergence).
func CheckOutOfScope(spec string, touchedPaths []string) error {
	entries := OutOfScopeEntries(spec)
	if len(entries) == 0 {
		return nil
	}
	for _, p := range touchedPaths {
		for _, e := range entries {
			entry := strings.TrimSuffix(strings.TrimSpace(e), "/")
			if entry == "" {
				continue
			}
			// Strip surrounding prose: take the path-looking token.
			for _, tok := range strings.Fields(entry) {
				tok = strings.Trim(tok, "`'\".,;:")
				if tok == "" || !strings.Contains(tok, "/") {
					continue
				}
				if p == tok || strings.HasPrefix(p, tok+"/") || strings.HasPrefix(p, strings.TrimPrefix(tok, "/")) {
					return fmt.Errorf("output touches out-of-scope path %q (spec declares %q out of scope)", p, tok)
				}
			}
		}
	}
	return nil
}

// Consolidate merges update into spec instead of appending: matching
// section headers get their content replaced; genuinely new sections are
// appended only while the result stays under the cap. Over-cap results
// are rejected — the caller must consolidate harder (spec 9.6).
func Consolidate(spec, update string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultSpecMaxBytes
	}
	merged := mergeSections(spec, update)
	if err := ValidateSpec(merged, maxBytes); err != nil {
		return "", err
	}
	return merged, nil
}

var mdSectionRe = regexp.MustCompile(`(?m)^(#{1,3}\s*.+)$`)

// mergeSections replaces existing sections by header text and appends
// only sections that don't already exist.
func mergeSections(spec, update string) string {
	upSections := splitSections(update)
	if len(upSections) == 0 {
		return spec
	}
	out := spec
	for _, u := range upSections {
		head := u[0]
		loc := findSectionHead(out, head)
		if loc < 0 {
			out = strings.TrimRight(out, "\n") + "\n\n" + strings.Join(u, "\n")
			continue
		}
		// Replace the existing section body up to the next same-level head.
		end := len(out)
		tail := out[loc+len(head):]
		if m := mdSectionRe.FindStringIndex(tail); m != nil {
			end = loc + len(head) + m[0]
		}
		out = out[:loc] + strings.Join(u, "\n") + "\n" + out[end:]
	}
	return out
}

// splitSections returns the update as [head, body...] groups.
func splitSections(text string) [][]string {
	locs := mdSectionRe.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return nil
	}
	var out [][]string
	for i, loc := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := strings.TrimSpace(text[loc[0]:end])
		lines := strings.SplitN(block, "\n", 2)
		head := strings.TrimSpace(lines[0])
		body := ""
		if len(lines) > 1 {
			body = strings.TrimSpace(lines[1])
		}
		out = append(out, []string{head, body})
	}
	return out
}

func findSectionHead(spec, head string) int {
	idx := 0
	for {
		i := mdSectionRe.FindStringIndex(spec[idx:])
		if i == nil {
			return -1
		}
		start := idx + i[0]
		lineEnd := idx + i[1]
		if strings.EqualFold(strings.TrimSpace(spec[start:lineEnd]), head) {
			return start
		}
		idx = lineEnd
	}
}
