package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPortIsNov12(t *testing.T) {
	if got := Default().Port; got != 1112 {
		t.Fatalf("default port = %d, want 1112", got)
	}
}

func TestLoadCreatesDefaultFileWhenMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mdpls")
	cfg, created, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created = true on first load")
	}
	if cfg.Port != Default().Port {
		t.Fatalf("port = %d, want default", cfg.Port)
	}
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	var onDisk Config
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	if onDisk.Theme.Accent == "" {
		t.Fatal("written file is missing theme values")
	}
}

func TestLoadMergesPartialFileOverDefaults(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"port": 4000, "theme": {"accent": "#00ff88"}}`)
	cfg, created, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("created should be false for an existing file")
	}
	if cfg.Port != 4000 || cfg.Theme.Accent != "#00ff88" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.Theme.Background != Default().Theme.Background {
		t.Fatalf("unset field should keep default, got %q", cfg.Theme.Background)
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"invalid json":      `{"port": `,
		"port too high":     `{"port": 70000}`,
		"css injection":     `{"theme": {"accent": "red;}</style><script>"}}`,
		"unknown codestyle": `{"theme": {"code_style": "no-such-style"}}`,
		"bad color scheme":  `{"theme": {"color_scheme": "purple"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, body)
			if _, _, err := Load(dir); err == nil || !strings.Contains(err.Error(), FileName) {
				t.Fatalf("Load error = %v, want an error naming %s", err, FileName)
			}
		})
	}
}

func TestDirHonoursEnv(t *testing.T) {
	t.Setenv("MDPLS_HOME", "/tmp/custom-mdpls")
	got, err := Dir()
	if err != nil || got != "/tmp/custom-mdpls" {
		t.Fatalf("Dir() = %q, %v", got, err)
	}
	t.Setenv("MDPLS_HOME", "")
	home, _ := os.UserHomeDir()
	got, err = Dir()
	if err != nil || got != filepath.Join(home, ".mdpls") {
		t.Fatalf("Dir() = %q, %v, want ~/.mdpls", got, err)
	}
}

func write(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
