package path

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinDirHonorsPKGLINE_BIN(t *testing.T) {
	t.Setenv("PKGLINE_ROOT", filepath.Join(t.TempDir(), "root"))
	custom := filepath.Join(t.TempDir(), "custom-bin")
	t.Setenv("PKGLINE_BIN", custom)
	if got := BinDir(); got != custom {
		t.Fatalf("BinDir() = %q, want %q", got, custom)
	}
}

func TestAppsDirHonorsPKGLINE_APPS(t *testing.T) {
	t.Setenv("PKGLINE_ROOT", filepath.Join(t.TempDir(), "root"))
	custom := filepath.Join(t.TempDir(), "custom-apps")
	t.Setenv("PKGLINE_APPS", custom)
	if got := AppsDir(); got != custom {
		t.Fatalf("AppsDir() = %q, want %q", got, custom)
	}
}

func TestBinDirDefaultsUnderRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	t.Setenv("PKGLINE_ROOT", root)
	t.Setenv("PKGLINE_BIN", "")
	t.Setenv("PKGLINE_APPS", "")
	if got := BinDir(); got != filepath.Join(root, "bin") {
		t.Fatalf("BinDir() = %q, want %q", got, filepath.Join(root, "bin"))
	}
	if got := AppsDir(); got != filepath.Join(root, "apps") {
		t.Fatalf("AppsDir() = %q, want %q", got, filepath.Join(root, "apps"))
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if got := ExpandPath("no-tilde"); got != "no-tilde" {
		t.Fatalf("plain path: %q", got)
	}
	if got := ExpandPath("~"); got != home {
		t.Fatalf("~ = %q, want %q", got, home)
	}
	want := filepath.Join(home, "x")
	if got := ExpandPath("~/x"); got != want {
		t.Fatalf("~/x = %q, want %q", got, want)
	}
}

func TestConfigDirAndDerivedPaths(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(t.TempDir(), "cfg")
	t.Setenv("PKGLINE_ROOT", root)
	t.Setenv("PKGLINE_CONFIG_DIR", cfg)
	t.Setenv("PKGLINE_BIN", "")
	t.Setenv("PKGLINE_APPS", "")

	if got := ConfigDir(); got != cfg {
		t.Fatalf("ConfigDir = %q", got)
	}
	if got := ConfigPath(); got != filepath.Join(cfg, "config.toml") {
		t.Fatalf("ConfigPath = %q", got)
	}
	if got := CacheDir(); got != filepath.Join(root, "cache") {
		t.Fatalf("CacheDir = %q", got)
	}
	if got := InventoryPath(); got != filepath.Join(root, "inventory.json") {
		t.Fatalf("InventoryPath = %q", got)
	}
	if got := AppPath("foo"); got != filepath.Join(root, "apps", "foo") {
		t.Fatalf("AppPath = %q", got)
	}
}

func TestConfigDirUsesXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("PKGLINE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got := ConfigDir(); got != filepath.Join(xdg, "pkgline") {
		t.Fatalf("ConfigDir via XDG = %q", got)
	}
}

func TestEnsureDirs(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(t.TempDir(), "cfg")
	t.Setenv("PKGLINE_ROOT", root)
	t.Setenv("PKGLINE_CONFIG_DIR", cfg)
	t.Setenv("PKGLINE_BIN", "")
	t.Setenv("PKGLINE_APPS", "")
	if err := EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{BinDir(), AppsDir(), CacheDir(), ConfigDir()} {
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			t.Fatalf("%s: %v", dir, err)
		}
	}
}
