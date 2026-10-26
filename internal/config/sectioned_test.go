package config

import (
	"os"
	"path/filepath"
	"testing"
)

// gate.min_mutants is owner-tunable: below the count the mutation sample
// is inconclusive and cannot veto (v0.9 acceptance rule 4).
func TestLayerKnobs_GateMinMutants(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ACE_CONFIG_DIR", dir)
	if err := os.MkdirAll(filepath.Join(dir, "config.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "[gate]\nmin_mutants = 5\n"
	if err := os.WriteFile(filepath.Join(dir, "config.d", "gate.toml"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	k := LoadLayerKnobs()
	if k.MutationMinMutants != 5 {
		t.Fatalf("got %d, want 5", k.MutationMinMutants)
	}
}

func TestLayerKnobs_GateMinMutantsDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ACE_CONFIG_DIR", dir)
	if k := LoadLayerKnobs(); k.MutationMinMutants != 3 {
		t.Fatalf("default got %d, want 3", k.MutationMinMutants)
	}
}
