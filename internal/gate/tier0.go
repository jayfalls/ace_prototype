package gate

import (
	"fmt"
	"strings"
)

// ---- shell command parsing -------------------------------------------------

// parseShell tokenizes a shell command into its constituent simple commands,
// unwrapping pipelines, lists, `sh -c`/`bash -c` bodies, $(...) and backtick
// substitutions. Matching happens on parsed argv, never raw text. Any parse
// failure returns an error and the caller escalates to Tier 2.
func parseShell(cmd string) ([][]string, error) {
	if strings.TrimSpace(cmd) == "" {
		return nil, fmt.Errorf("empty command")
	}
	var cmds [][]string
	if err := splitShell(cmd, &cmds, 0); err != nil {
		return nil, err
	}
	return cmds, nil
}

// splitShell walks the string, splitting on unquoted | ; && || newlines and
// recursing into $(...) / `...` / sh -c "..." segments. depth bounds recursion.
func splitShell(s string, out *[][]string, depth int) error {
	if depth > 8 {
		return fmt.Errorf("nesting too deep")
	}
	var cur strings.Builder
	var inS, inD bool // single/double quote state
	runes := []rune(s)
	push := func() error {
		seg := strings.TrimSpace(cur.String())
		cur.Reset()
		if seg == "" {
			return nil
		}
		argv, err := tokenize(seg)
		if err != nil {
			return err
		}
		if len(argv) > 0 {
			*out = append(*out, argv)
		}
		return nil
	}
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case inS:
			// Quote chars pass through so `sh -c "…"` survives as one argv
			// element for the wrapper unwrapping step; tokenize re-strips.
			if c == '\'' {
				inS = false
			}
			cur.WriteRune(c)
		case inD:
			if c == '"' {
				inD = false
			}
			cur.WriteRune(c)
		case c == '\'':
			inS = true
			cur.WriteRune(c)
		case c == '"':
			inD = true
			cur.WriteRune(c)
		case c == '$' && i+1 < len(runes) && runes[i+1] == '(':
			// $(...) — find matching close paren, recurse inside
			depth2 := 0
			j := i + 1
			for ; j < len(runes); j++ {
				if runes[j] == '(' {
					depth2++
				} else if runes[j] == ')' {
					depth2--
					if depth2 == 0 {
						break
					}
				}
			}
			if j >= len(runes) {
				return fmt.Errorf("unbalanced $(...)")
			}
			if err := splitShell(string(runes[i+2:j]), out, depth+1); err != nil {
				return err
			}
			i = j
		case c == '`':
			j := i + 1
			for ; j < len(runes) && runes[j] != '`'; j++ {
			}
			if j >= len(runes) {
				return fmt.Errorf("unbalanced backtick")
			}
			if err := splitShell(string(runes[i+1:j]), out, depth+1); err != nil {
				return err
			}
			i = j
		case c == '&' && i+1 < len(runes) && runes[i+1] == '&':
			if err := push(); err != nil {
				return err
			}
			i++
		case c == '|' && i+1 < len(runes) && runes[i+1] == '|':
			if err := push(); err != nil {
				return err
			}
			i++
		case c == '|' || c == ';' || c == '\n':
			if err := push(); err != nil {
				return err
			}
		default:
			cur.WriteRune(c)
		}
	}
	if inS || inD {
		return fmt.Errorf("unbalanced quote")
	}
	return push()
}

// tokenize splits one simple command into argv (quotes already stripped at the
// splitter level for parsing — here we keep quoting minimal: whitespace split,
// then unwrap sh -c / bash -c / env bodies recursively at the caller level via
// unwrapWrappers).
func tokenize(seg string) ([]string, error) {
	var argv []string
	var cur strings.Builder
	var inS, inD bool
	flush := func() {
		if cur.Len() > 0 {
			argv = append(argv, cur.String())
			cur.Reset()
		}
	}
	for _, c := range seg {
		switch {
		case inS:
			if c == '\'' {
				inS = false
			} else {
				cur.WriteRune(c)
			}
		case inD:
			if c == '"' {
				inD = false
			} else {
				cur.WriteRune(c)
			}
		case c == '\'':
			inS = true
		case c == '"':
			inD = true
		case c == ' ' || c == '\t':
			flush()
		default:
			cur.WriteRune(c)
		}
	}
	if inS || inD {
		return nil, fmt.Errorf("unbalanced quote in %q", seg)
	}
	flush()
	return argv, nil
}

// unwrapShellWrappers returns the inner command string when the command is a
// `sh -c "..."` / `bash -c "..."` / `env ... sh -c "..."` wrapper, else "".
func unwrapShellWrapper(argv []string) string {
	// skip env VAR=val prefixes
	for len(argv) > 0 && (argv[0] == "env" || strings.Contains(argv[0], "=")) {
		argv = argv[1:]
		if len(argv) > 0 && strings.HasPrefix(argv[0], "-") { // env flags
			argv = argv[1:]
		}
	}
	if len(argv) >= 3 {
		base := argv[0]
		if b := strings.LastIndexByte(base, '/'); b >= 0 {
			base = base[b+1:]
		}
		if (base == "sh" || base == "bash" || base == "zsh" || base == "dash") && (argv[1] == "-c" || argv[1] == "-lc" || argv[1] == "-ec") {
			return argv[2]
		}
	}
	return ""
}

// ---- Tier 0 rules ----------------------------------------------------------

// protectedTargets are paths no layer may write through tools or shell —
// constitution and owner config are outside the jail and read-only by law.
var protectedTargets = []string{"soul.md", "mission.md", "constitution", "personality.txt", "grants.json", "models.json", "bearer_token"}

// tier0 decides deterministic pass/deny, or returns Verdict "" to fall through.
// Unparseable shell returns an explicit escalate marker decision.
func (g *Gate) tier0(a Action) Decision {
	kind := strings.ToLower(a.Kind)

	// Reads inside the jail pass.
	if kind == "workspace_read" || kind == "wiki_read" || kind == "memory_search" {
		return Decision{Verdict: VerdictAllow, Tier: 0, RuleID: "t0.read-in-jail"}
	}
	// Writes are checked against protected paths first, then workpad scope.
	if kind == "workspace_write" || kind == "workspace_edit" || kind == "wiki_write" {
		for _, p := range protectedTargets {
			if strings.Contains(strings.ToLower(a.Target), p) {
				return Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-protected-path", Reason: "target touches protected constitution/config path"}
			}
		}
		if strings.HasPrefix(a.Target, "/workspace/") || strings.HasPrefix(a.Target, "workspace/") || kind == "wiki_write" {
			return Decision{Verdict: VerdictAllow, Tier: 0, RuleID: "t0.workpad-write"}
		}
		return Decision{Tier: -1} // outside workpad — escalate to fast gate
	}
	if kind == "sandbox_sh" || kind == "shell" || kind == "exec" {
		return g.tier0Shell(a)
	}
	return Decision{Tier: -1} // not a Tier-0 kind
}

// tier0Shell parses the command and applies the deny list per argv.
func (g *Gate) tier0Shell(a Action) Decision {
	cmds, err := parseShell(a.Command)
	if err != nil {
		return Decision{Verdict: "escalate", Tier: 0, RuleID: "t0.unparseable", Reason: err.Error()}
	}
	// unwrap sh -c / bash -c wrappers into the parse set
	for i := 0; i < len(cmds); i++ {
		if inner := unwrapShellWrapper(cmds[i]); inner != "" {
			if sub, err := parseShell(inner); err == nil {
				cmds = append(cmds, sub...)
			} else {
				return Decision{Verdict: "escalate", Tier: 0, RuleID: "t0.unparseable-wrapper", Reason: err.Error()}
			}
		}
	}
	if d := matchDenyList(cmds); d != nil {
		return *d
	}
	return Decision{Tier: -1} // parsed clean — Tier 1 decides
}

// matchDenyList applies §5 deny rules to parsed argv commands.
func matchDenyList(cmds [][]string) *Decision {
	for _, argv := range cmds {
		if len(argv) == 0 {
			continue
		}
		bin := argv[0]
		if b := strings.LastIndexByte(bin, '/'); b >= 0 {
			bin = bin[b+1:]
		}
		if strings.HasPrefix(bin, "mkfs") {
			return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-blockdev", Reason: bin}
		}
		switch bin {
		case "sudo", "su", "doas":
			return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-privesc", Reason: bin}
		case "mke2fs", "fdisk", "wipefs":
			return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-blockdev", Reason: bin}
		case "shutdown", "reboot", "poweroff", "systemctl", "useradd", "userdel":
			return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-system", Reason: bin}
		case "rm":
			if rmOutsideWorkspace(argv) {
				return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-rm-outside", Reason: strings.Join(argv, " ")}
			}
		case "dd":
			for _, arg := range argv[1:] {
				if strings.HasPrefix(arg, "of=/dev/") {
					return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-dd-device", Reason: arg}
				}
			}
		case "git":
			if d := denyGit(argv); d != nil {
				return d
			}
		case "cat", "less", "head", "tail", "cp", "scp":
			for _, arg := range argv[1:] {
				if isSecretPath(arg) {
					return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-secrets", Reason: arg}
				}
			}
		}
	}
	// curl/wget piped into a shell interpreter in the same command line
	var hasFetch, hasInterp bool
	for _, argv := range cmds {
		if len(argv) == 0 {
			continue
		}
		b := argv[0]
		if i := strings.LastIndexByte(b, '/'); i >= 0 {
			b = b[i+1:]
		}
		switch b {
		case "curl", "wget":
			hasFetch = true
		case "sh", "bash", "zsh", "dash", "python", "python3", "perl":
			hasInterp = true
		}
	}
	if hasFetch && hasInterp {
		return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-net-pipe", Reason: "remote content piped to interpreter"}
	}
	return nil
}

func rmOutsideWorkspace(argv []string) bool {
	var recursive bool
	for _, a := range argv[1:] {
		if strings.HasPrefix(a, "-") {
			if strings.ContainsAny(a, "rR") {
				recursive = true
			}
			continue
		}
		if recursive && !strings.HasPrefix(a, "/workspace") && !strings.HasPrefix(a, "workspace/") && !strings.HasPrefix(a, ".") {
			return true
		}
	}
	return false
}

func denyGit(argv []string) *Decision {
	if len(argv) < 2 {
		return nil
	}
	sub := argv[1]
	if sub == "push" {
		force := false
		for _, a := range argv[2:] {
			if a == "--force" || a == "-f" || a == "--force-with-lease" {
				force = true
			}
			if a == "main" || a == "master" || strings.HasSuffix(a, ":main") || strings.HasSuffix(a, ":master") {
				return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-git-main-push", Reason: "push to protected main is owner-only (PR flow)"}
			}
		}
		if force {
			return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-git-force-push", Reason: "force push"}
		}
	}
	if sub == "config" {
		for _, a := range argv[2:] {
			if a == "--global" || a == "--system" {
				return &Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "t0.deny-gitconfig-global"}
			}
		}
	}
	return nil
}

func isSecretPath(p string) bool {
	lp := strings.ToLower(p)
	return strings.Contains(lp, "/.ssh/") || strings.HasPrefix(lp, "~/.ssh") ||
		lp == "/etc/shadow" || lp == "/etc/gshadow" ||
		(strings.HasSuffix(lp, ".pem") && !strings.HasPrefix(lp, "/workspace"))
}
