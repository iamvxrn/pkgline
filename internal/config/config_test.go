package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveURI(t *testing.T) {
	cfg := &Config{
		Aliases: map[string]string{
			"cbld": "gh:iamvxrn/cbld",
			"mine": "  gl:group/proj  ",
		},
	}

	tests := []struct {
		in, want string
	}{
		{"cbld", "https://github.com/iamvxrn/cbld.git"},
		{"mine", "https://gitlab.com/group/proj.git"},
		{"gh:user/repo", "https://github.com/user/repo.git"},
		{"gh:/user/repo/", "https://github.com/user/repo.git"},
		{"gl:group/proj", "https://gitlab.com/group/proj.git"},
		{"cb:user/repo", "https://codeberg.org/user/repo.git"},
		{"sh:user/repo", "https://git.sr.ht/~user/repo.git"},
		{"sh:~already/repo", "https://git.sr.ht/~already/repo.git"},
		{"owner/repo", "https://github.com/owner/repo.git"},
		{"https://example.com/a.git", "https://example.com/a.git"},
		{"git@github.com:a/b.git", "git@github.com:a/b.git"},
		{"./local", "./local"},
		{"/abs/path", "/abs/path"},
		{"  gh:x/y  ", "https://github.com/x/y.git"},
	}
	for _, tt := range tests {
		if got := cfg.ResolveURI(tt.in); got != tt.want {
			t.Errorf("ResolveURI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PKGLINE_CONFIG_DIR", dir)
	t.Setenv("PKGLINE_ROOT", filepath.Join(dir, "root"))
	t.Setenv("PKGLINE_BIN", "")
	t.Setenv("PKGLINE_APPS", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Aliases["pkgline"] != "gh:iamvxrn/pkgline" {
		t.Fatalf("default alias missing: %#v", cfg.Aliases)
	}
	if cfg.BinDir == "" || cfg.AppsDir == "" {
		t.Fatalf("expected default dirs, got bin=%q apps=%q", cfg.BinDir, cfg.AppsDir)
	}
}

func TestLoadConfigMergesFileOverDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PKGLINE_CONFIG_DIR", dir)
	customBin := filepath.Join(dir, "bin")
	customApps := filepath.Join(dir, "apps")
	toml := fmt.Sprintf("bin_dir = %q\napps_dir = %q\n[aliases]\nmytool = \"gh:foo/bar\"\n", customBin, customApps)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.BinDir != customBin {
		t.Errorf("BinDir = %q, want %q", cfg.BinDir, customBin)
	}
	if cfg.AppsDir != customApps {
		t.Errorf("AppsDir = %q, want %q", cfg.AppsDir, customApps)
	}
	if cfg.Aliases["mytool"] != "gh:foo/bar" {
		t.Errorf("file alias missing: %#v", cfg.Aliases)
	}
	if cfg.Aliases["cbld"] != "gh:iamvxrn/cbld" {
		t.Errorf("default alias dropped: %#v", cfg.Aliases)
	}
}

func TestLoadConfigEnvWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PKGLINE_CONFIG_DIR", dir)
	envBin := filepath.Join(dir, "env-bin")
	fileBin := filepath.Join(dir, "file-bin")
	t.Setenv("PKGLINE_BIN", envBin)
	t.Setenv("PKGLINE_APPS", "")
	toml := fmt.Sprintf("bin_dir = %q\n", fileBin)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BinDir != envBin {
		t.Fatalf("BinDir = %q, want env %q", cfg.BinDir, envBin)
	}
}

func TestLoadConfigRejectsInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PKGLINE_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[[[not toml"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected parse error")
	}
}
