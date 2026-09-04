package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writePkg(t *testing.T, dir, manifestBody string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	must("pkgline.toml", manifestBody)
	must("go.mod", "module example.com/tool\n\ngo 1.20\n")
	must("main.go", "package main\nfunc main(){}\n")
}

// Regression: a hostile pkgline.toml used to make Install RemoveAll a directory
// outside AppsDir and write its binary outside BinDir.
func TestInstallRejectsTraversingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatal(err)
	}

	victim := filepath.Join(tmpDir, ".pkgline", "victim")
	if err := os.MkdirAll(victim, 0755); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(victim, "precious.txt")
	if err := os.WriteFile(precious, []byte("do not delete"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "evil-pkg")
	writePkg(t, pkgDir, `
[package]
name = "../victim"
version = "0.1.0"
language = "go"
executable = "../../pwned-binary"
`)

	if err := installer.Install(pkgDir); err == nil {
		t.Fatal("Install accepted a manifest whose name escapes AppsDir")
	}
	if _, err := os.Stat(precious); err != nil {
		t.Errorf("install destroyed a directory outside AppsDir: %v", err)
	}
	for _, escaped := range []string{
		filepath.Join(tmpDir, "pwned-binary"),
		filepath.Join(tmpDir, ".pkgline", "pwned-binary"),
	} {
		if _, err := os.Stat(escaped); err == nil {
			t.Errorf("binary written outside BinDir at %s", escaped)
		}
	}
}

// Regression: `pkgline remove ../..` built a path outside AppsDir and RemoveAll'd it.
func TestRemoveRejectsTraversingName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(tmpDir, ".pkgline", "victim")
	if err := os.MkdirAll(victim, 0755); err != nil {
		t.Fatal(err)
	}
	if err := installer.Remove("../victim"); err == nil {
		t.Fatal("Remove accepted a traversing package name")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("Remove deleted a directory outside AppsDir: %v", err)
	}
}

// A natively-built package must not run its [scripts] uninstall hook: that hook
// exists to undo the install hook, which the native path never ran. Both cbld
// and vibeporter ship one that deletes ~/.local/bin/<bin>, a file pkgline never
// created.
func TestRemoveSkipsUninstallHookForNativeBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh script")
	}
	tmpDir := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(tmpDir, ".pkgline"))
	t.Setenv("PKGLINE_CONFIG_DIR", filepath.Join(tmpDir, ".config", "pkgline"))

	installer, err := NewInstaller()
	if err != nil {
		t.Fatal(err)
	}

	// Stands in for ~/.local/bin/<bin>: outside pkgline, not pkgline's to delete.
	outside := filepath.Join(tmpDir, "local-bin-tool")
	if err := os.WriteFile(outside, []byte("preexisting"), 0755); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "native-pkg")
	writePkg(t, pkgDir, `
[package]
name = "nativetool"
version = "0.1.0"
language = "go"
executable = "nativetool"

[scripts]
install = "install.sh"
uninstall = "uninstall.sh"
`)
	// Mirrors the sibling projects: hardcoded target, ignores PKGLINE_BIN.
	if err := os.WriteFile(filepath.Join(pkgDir, "uninstall.sh"),
		[]byte("#!/bin/sh\nrm -f "+outside+"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "install.sh"), []byte("#!/bin/sh\ntrue\n"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := installer.Install(pkgDir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := installer.Remove("nativetool"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("uninstall hook ran for a natively built package and deleted %s", outside)
	}
}
