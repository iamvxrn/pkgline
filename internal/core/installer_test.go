package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"pkgline/internal/db"
	"pkgline/internal/path"
)

func TestSmartInstallationGo(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatalf("NewInstaller error: %v", err)
	}

	// Create mock Go package directory
	pkgDir := filepath.Join(tmpDir, "mock-go-pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	// Write pkgline.toml
	manifestContent := `
[package]
name = "mock-go-tool"
version = "0.1.0"
language = "go"
executable = "mock-go-tool"
`
	if err := os.WriteFile(filepath.Join(pkgDir, "pkgline.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write pkgline.toml: %v", err)
	}

	// Write mock main.go & go.mod
	if err := os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte("module mock-go-tool\n\ngo 1.20\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	mainGoContent := `package main
import "fmt"
func main() { fmt.Println("mock go tool") }
`
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte(mainGoContent), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Install package
	if err := installer.Install(pkgDir); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify binary exists
	expectedExe := "mock-go-tool"
	if runtime.GOOS == "windows" {
		expectedExe += ".exe"
	}
	binPath := filepath.Join(installer.cfg.BinDir, expectedExe)
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Errorf("binary was not created at expected path: %s", binPath)
	}

	// Remove package
	if err := installer.Remove("mock-go-tool"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify binary was removed
	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Errorf("binary was not removed from path: %s", binPath)
	}
}

func TestSmartInstallationScriptFallback(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatalf("NewInstaller error: %v", err)
	}

	// Create mock script package directory
	pkgDir := filepath.Join(tmpDir, "mock-script-pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	// Write pkgline.toml
	manifestContent := `
[package]
name = "mock-script-tool"
version = "0.1.0"
executable = "mock-script-tool"

[scripts]
install = "install.sh"
uninstall = "uninstall.sh"
`
	if err := os.WriteFile(filepath.Join(pkgDir, "pkgline.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write pkgline.toml: %v", err)
	}

	// Write mock install.sh
	installShContent := `#!/bin/sh
echo "Installing script tool to $PKGLINE_BIN"
touch "$PKGLINE_BIN/$PKGLINE_EXECUTABLE"
chmod +x "$PKGLINE_BIN/$PKGLINE_EXECUTABLE"
`
	if err := os.WriteFile(filepath.Join(pkgDir, "install.sh"), []byte(installShContent), 0755); err != nil {
		t.Fatalf("failed to write install.sh: %v", err)
	}

	// Write mock uninstall.sh
	uninstallShContent := `#!/bin/sh
echo "Uninstalling script tool from $PKGLINE_BIN"
rm -f "$PKGLINE_BIN/$PKGLINE_EXECUTABLE"
`
	if err := os.WriteFile(filepath.Join(pkgDir, "uninstall.sh"), []byte(uninstallShContent), 0755); err != nil {
		t.Fatalf("failed to write uninstall.sh: %v", err)
	}

	// Install package
	if err := installer.Install(pkgDir); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify binary exists
	expectedExe := "mock-script-tool"
	if runtime.GOOS == "windows" {
		expectedExe += ".exe"
	}
	binPath := filepath.Join(installer.cfg.BinDir, expectedExe)
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Errorf("binary was not created at expected path: %s", binPath)
	}

	// Remove package
	if err := installer.Remove("mock-script-tool"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify binary was removed
	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Errorf("binary was not removed from path: %s", binPath)
	}
}

func TestSmartInstallationValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatalf("NewInstaller error: %v", err)
	}

	// Create mock invalid package directory (no native source, no install script)
	pkgDir := filepath.Join(tmpDir, "mock-invalid-pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	manifestContent := `
[package]
name = "mock-invalid-tool"
version = "0.1.0"
language = "python"
`
	if err := os.WriteFile(filepath.Join(pkgDir, "pkgline.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write pkgline.toml: %v", err)
	}

	// Attempt install, expect error
	if err := installer.Install(pkgDir); err == nil {
		t.Errorf("expected Install to fail for invalid package manifest")
	}
}

func TestConfigResolveURI(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	cfgPath := path.ConfigPath()
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0755)

	tomlContent := `
[aliases]
mytool = "gh:foo/bar"
`
	_ = os.WriteFile(cfgPath, []byte(tomlContent), 0644)

	installer, err := NewInstaller()
	if err != nil {
		t.Fatalf("NewInstaller error: %v", err)
	}

	resolved := installer.cfg.ResolveURI("mytool")
	expected := "https://github.com/foo/bar.git"
	if resolved != expected {
		t.Errorf("alias resolution failed: expected %s, got %s", expected, resolved)
	}

	resolvedCbld := installer.cfg.ResolveURI("cbld")
	expectedCbld := "https://github.com/iamvxrn/cbld.git"
	if resolvedCbld != expectedCbld {
		t.Errorf("cbld built-in alias failed: expected %s, got %s", expectedCbld, resolvedCbld)
	}

	resolvedMuth := installer.cfg.ResolveURI("muth")
	expectedMuth := "https://github.com/iamvxrn/muth.git"
	if resolvedMuth != expectedMuth {
		t.Errorf("muth built-in alias failed: expected %s, got %s", expectedMuth, resolvedMuth)
	}

	resolvedRuna := installer.cfg.ResolveURI("runa")
	expectedRuna := "https://github.com/iamvxrn/runa.git"
	if resolvedRuna != expectedRuna {
		t.Errorf("runa built-in alias failed: expected %s, got %s", expectedRuna, resolvedRuna)
	}

	resolvedGH := installer.cfg.ResolveURI("gh:user/repo")
	expectedGH := "https://github.com/user/repo.git"
	if resolvedGH != expectedGH {
		t.Errorf("gh: resolution failed: expected %s, got %s", expectedGH, resolvedGH)
	}

	if got := installer.cfg.ResolveURI("gl:group/proj"); got != "https://gitlab.com/group/proj.git" {
		t.Errorf("gl: resolution failed: got %s", got)
	}
	if got := installer.cfg.ResolveURI("cb:user/repo"); got != "https://codeberg.org/user/repo.git" {
		t.Errorf("cb: resolution failed: got %s", got)
	}
	if got := installer.cfg.ResolveURI("sh:user/repo"); got != "https://git.sr.ht/~user/repo.git" {
		t.Errorf("sh: resolution failed: got %s", got)
	}
	if got := installer.cfg.ResolveURI("sh:~already/repo"); got != "https://git.sr.ht/~already/repo.git" {
		t.Errorf("sh:~ resolution failed: got %s", got)
	}
}

func TestRollbackListSyncRemove(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatal(err)
	}

	if err := installer.Sync(""); err != nil {
		t.Fatalf("Sync empty inventory: %v", err)
	}
	if err := installer.Remove("nope"); err == nil {
		t.Fatal("expected Remove of missing package to fail")
	}
	if err := installer.Rollback("nope"); err == nil {
		t.Fatal("expected Rollback of missing package to fail")
	}

	exe := "rollback-tool"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	binPath := filepath.Join(path.BinDir(), exe)
	if err := os.WriteFile(binPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath+".bak", []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := installer.store.Set(db.PackageRecord{
		Name:       "rollback-tool",
		Version:    "1.0",
		Executable: exe,
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeRollbackMeta(binPath, db.PackageRecord{
		Name:       "rollback-tool",
		Version:    "0.9",
		Executable: exe,
	}); err != nil {
		t.Fatal(err)
	}
	if err := installer.Rollback("rollback-tool"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binPath)
	if err != nil || string(got) != "old" {
		t.Fatalf("rolled back content = %q err=%v", got, err)
	}
	rec, ok, err := installer.store.Get("rollback-tool")
	if err != nil || !ok || rec.Version != "0.9" {
		t.Fatalf("inventory after rollback: %+v ok=%v err=%v", rec, ok, err)
	}

	if err := installer.List(true); err != nil {
		t.Fatal(err)
	}
	if err := installer.List(false); err != nil {
		t.Fatal(err)
	}
}

func TestShortenHashAndCopyHelpers(t *testing.T) {
	if got := shortenHash("abcdefghij"); got != "abcdefg" {
		t.Fatalf("shortenHash = %q", got)
	}
	if got := shortenHash("abc"); got != "abc" {
		t.Fatalf("short hash = %q", got)
	}

	src := filepath.Join(t.TempDir(), "f")
	dst := filepath.Join(t.TempDir(), "g")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	from := t.TempDir()
	to := filepath.Join(t.TempDir(), "copy")
	if err := os.WriteFile(filepath.Join(from, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copyDir(from, to); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(to, "a.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallHonorsConfigBinDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, "cfg")
	customBin := filepath.Join(tmpDir, "mybin")
	customApps := filepath.Join(tmpDir, "myapps")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", cfgDir)
	t.Setenv("PKGLINE_BIN", "")
	t.Setenv("PKGLINE_APPS", "")
	toml := "bin_dir = \"" + filepath.ToSlash(customBin) + "\"\napps_dir = \"" + filepath.ToSlash(customApps) + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	installer, err := NewInstaller()
	if err != nil {
		t.Fatal(err)
	}
	if installer.cfg.BinDir != customBin && installer.cfg.BinDir != filepath.Clean(customBin) {
		// TOML may keep slash form; compare cleaned
		if filepath.Clean(installer.cfg.BinDir) != filepath.Clean(customBin) {
			t.Fatalf("cfg.BinDir = %q want %q", installer.cfg.BinDir, customBin)
		}
	}

	pkgDir := filepath.Join(tmpDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "pkgline.toml"), []byte(`
[package]
name = "cfgbin"
version = "0.1.0"
language = "go"
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte("module cfgbin\n\ngo 1.20\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := installer.Install(pkgDir); err != nil {
		t.Fatal(err)
	}
	exe := "cfgbin"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if _, err := os.Stat(filepath.Join(installer.cfg.BinDir, exe)); err != nil {
		t.Fatalf("binary not in config bin_dir: %v", err)
	}
}

func TestSyncLocalInstallRebuildsOnVersionBump(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))
	t.Setenv("PKGLINE_BIN", "")
	t.Setenv("PKGLINE_APPS", "")

	installer, err := NewInstaller()
	if err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeGoPkg := func(ver string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(pkgDir, "pkgline.toml"), []byte(`
[package]
name = "sync-local"
version = "`+ver+`"
language = "go"
`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte("module synclocal\n\ngo 1.20\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeGoPkg("0.1.0")
	if err := installer.Install(pkgDir); err != nil {
		t.Fatal(err)
	}

	appToml := filepath.Join(installer.appPath("sync-local"), "pkgline.toml")
	if err := os.WriteFile(appToml, []byte(`
[package]
name = "sync-local"
version = "0.2.0"
language = "go"
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := installer.Sync("sync-local"); err != nil {
		t.Fatal(err)
	}
	rec, ok, err := installer.store.Get("sync-local")
	if err != nil || !ok {
		t.Fatalf("record: ok=%v err=%v", ok, err)
	}
	if rec.Version != "0.2.0" {
		t.Fatalf("version after sync = %q", rec.Version)
	}
	exe := rec.Executable
	bak := installer.binFile(exe) + ".bak"
	if _, err := os.Stat(bak); err != nil {
		t.Fatalf("expected .bak after sync rebuild: %v", err)
	}
}

func TestNameFromURI(t *testing.T) {
	cases := map[string]string{
		"https://github.com/user/cooltool.git": "cooltool",
		"gh:user/repo":                         "repo",
		"/tmp/my-pkg":                          "my-pkg",
		"my-pkg":                               "my-pkg",
	}
	for in, want := range cases {
		if got := nameFromURI(in); got != want {
			t.Errorf("nameFromURI(%q) = %q want %q", in, got, want)
		}
	}
}

func TestInstallWithOverridesLanguageAndExec(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "wrong-lang")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "pkgline.toml"), []byte(`
[package]
name = "wrong-lang"
version = "0.1.0"
language = "rust"
executable = "wrong-lang"
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte("module wrong-lang\n\ngo 1.20\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	wantExe := "overridden"
	if runtime.GOOS == "windows" {
		wantExe += ".exe"
	}
	if err := installer.InstallWith(pkgDir, InstallOpts{Language: "go", Executable: "overridden"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installer.cfg.BinDir, wantExe)); err != nil {
		t.Fatalf("binary: %v", err)
	}
	rec, ok, err := installer.store.Get("wrong-lang")
	if err != nil || !ok {
		t.Fatalf("record: ok=%v err=%v", ok, err)
	}
	if rec.Language != "go" {
		t.Fatalf("language = %q", rec.Language)
	}
	if rec.Executable != wantExe {
		t.Fatalf("executable = %q", rec.Executable)
	}
}
