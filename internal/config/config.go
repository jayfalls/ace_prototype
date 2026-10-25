// Package config holds ACE's owner-set configuration: the local bearer token now,
// mission/personality/capability grants and model roles from v0.8 (plan 8.1).
// Owner-set files are authoritative and read-only to the agent (plan §2).
package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dir returns the owner config directory: $ACE_CONFIG_DIR, else <user-config>/ace.
func Dir() string {
	if d := strings.TrimSpace(os.Getenv("ACE_CONFIG_DIR")); d != "" {
		return d
	}
	d, err := os.UserConfigDir()
	if err != nil || d == "" {
		return filepath.Join(os.TempDir(), "ace-config")
	}
	return filepath.Join(d, "ace")
}

// LoadOrCreateToken returns the local API bearer token: $ACE_BEARER_TOKEN if set,
// else a 256-bit token generated once and persisted at <config-dir>/bearer_token (0600).
func LoadOrCreateToken() (string, error) {
	if t := strings.TrimSpace(os.Getenv("ACE_BEARER_TOKEN")); t != "" {
		return t, nil
	}
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "bearer_token")
	if b, err := os.ReadFile(p); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(raw)
	if err := os.WriteFile(p, []byte(tok+"\n"), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

// ---- Owner-set files (plan 8.1): written only by `ace init` / owner edits,
// read-only to the agent. ----

const (
	MissionFile     = "mission.md"
	PersonalityFile = "personality.txt"
	GrantsFile      = "grants.json"
	ModelsFile      = "models.json"
)

// LoadMission returns the owner's life mission ("" when never configured —
// ACE must idle until it is). Read every cycle: the file is small and the
// content hash is the mission version.
func LoadMission() string {
	b, err := os.ReadFile(filepath.Join(Dir(), MissionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// MissionVersion is the content hash of mission.md — any owner edit produces a
// new version, and the L1 anchor + DAG provenance re-derive from it.
func MissionVersion() string {
	sum := sha256.Sum256([]byte(LoadMission()))
	return hex.EncodeToString(sum[:])[:12]
}

// WriteMission replaces mission.md — `ace init` and owner edits only.
func WriteMission(mission string) error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, MissionFile), []byte(strings.TrimSpace(mission)+"\n"), 0o600)
}

// LoadPersonality returns the optional ≤1KB owner-set personality text.
func LoadPersonality() string {
	b, err := os.ReadFile(filepath.Join(Dir(), PersonalityFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// WriteInit persists all owner-set files from `ace init`.
func WriteInit(mission, personality string, grants []string) error {
	if err := WriteMission(mission); err != nil {
		return err
	}
	dir := Dir()
	if err := os.WriteFile(filepath.Join(dir, PersonalityFile), []byte(strings.TrimSpace(personality)+"\n"), 0o600); err != nil {
		return err
	}
	if grants == nil {
		grants = []string{}
	}
	return os.WriteFile(filepath.Join(dir, GrantsFile), []byte(mustJSON(map[string]any{"grants": grants})), 0o600)
}

// LoadGrants returns the owner-granted capability scopes (empty = none granted).
func LoadGrants() []string {
	b, err := os.ReadFile(filepath.Join(Dir(), GrantsFile))
	if err != nil {
		return nil
	}
	var g struct {
		Grants []string `json:"grants"`
	}
	if json.Unmarshal(b, &g) != nil {
		return nil
	}
	return g.Grants
}

// ModelRoles is the minimal model-role assignment (plan 8.1): main drives
// L1–L6, fast covers Tier-1/cheap calls, ensemble is the different-families set
// used for L1 verdicts and voting (8.2).
type ModelRoles struct {
	Main     string   `json:"main"`
	Fast     string   `json:"fast"`
	Ensemble []string `json:"ensemble"`
}

// LoadModelRoles reads models.json; zero-valued fields are absent/failed reads.
func LoadModelRoles() ModelRoles {
	var m ModelRoles
	b, err := os.ReadFile(filepath.Join(Dir(), ModelsFile))
	if err != nil {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}

// WriteModelRoles persists models.json — owner-set.
func WriteModelRoles(m ModelRoles) error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ModelsFile), []byte(mustJSON(m)), 0o600)
}

// Validate enforces plan 8.1: a configured ensemble needs >=3 models from
// different families (family = model-id prefix before the first '-').
func (m ModelRoles) Validate() error {
	if len(m.Ensemble) == 0 {
		return nil
	}
	if len(m.Ensemble) < 3 {
		return fmt.Errorf("models.ensemble needs >=3 models, got %d", len(m.Ensemble))
	}
	fams := map[string]bool{}
	for _, id := range m.Ensemble {
		fam := strings.SplitN(strings.TrimSpace(id), "-", 2)[0]
		fams[fam] = true
	}
	if len(fams) < 3 {
		return fmt.Errorf("models.ensemble must span 3 different families, got %v", m.Ensemble)
	}
	return nil
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("config marshal: %v", err))
	}
	return string(b) + "\n"
}
