package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnv_ModeEnforced(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ACE_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("ACE_TEST_K=v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnv(); err == nil {
		t.Fatal("0644 .env must fail startup")
	}
}

func TestLoadEnv_SetsUnsetKeepsSet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ACE_CONFIG_DIR", dir)
	content := "# comment\nACE_TEST_SET=from_file\nACE_TEST_KEEP=file_loses\nACE_TEST_Q=\"quoted value\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ACE_TEST_KEEP", "env_wins")
	if err := LoadEnv(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ACE_TEST_SET") != "from_file" {
		t.Fatal("unset key not loaded")
	}
	if os.Getenv("ACE_TEST_KEEP") != "env_wins" {
		t.Fatal(".env overrode an already-set variable")
	}
	if os.Getenv("ACE_TEST_Q") != "quoted value" {
		t.Fatalf("quoted value not unwrapped: %q", os.Getenv("ACE_TEST_Q"))
	}
}

func TestLoadEnv_MissingOK(t *testing.T) {
	t.Setenv("ACE_CONFIG_DIR", t.TempDir())
	if err := LoadEnv(); err != nil {
		t.Fatal("missing .env must not error")
	}
}
