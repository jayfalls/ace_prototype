// env.go — owner-managed credentials (v0.9 §9.0): <config-dir>/.env is the
// only credential file ACE reads. The supervisor loads it before forking the
// worker, which inherits os.Environ().
package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnv reads <config-dir>/.env into the process environment. KEY=VALUE
// lines, '#' comments, optional single/double quotes. A missing file is not
// an error; a file readable by anyone but the owner fails startup.
// Already-set variables win. Values are never logged — key names only.
func LoadEnv() error {
	path := filepath.Join(Dir(), ".env")
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s must be mode 0600", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var loaded []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && v[0] == v[len(v)-1] && (v[0] == '"' || v[0] == '\'') {
			v = v[1 : len(v)-1]
		}
		if k == "" || os.Getenv(k) != "" {
			continue
		}
		_ = os.Setenv(k, v)
		loaded = append(loaded, k)
	}
	if len(loaded) > 0 {
		log.Printf("env: loaded keys [%s] from %s", strings.Join(loaded, " "), path)
	}
	return nil
}
