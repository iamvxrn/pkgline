package path

import (
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
