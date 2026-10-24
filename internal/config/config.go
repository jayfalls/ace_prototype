// Package config holds ACE's owner-set configuration: the local bearer token now,
// mission/personality/capability grants and model roles from v0.8 (plan 8.1).
// Owner-set files are authoritative and read-only to the agent (plan §2).
package config

import (
	"crypto/rand"
	"encoding/hex"
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
