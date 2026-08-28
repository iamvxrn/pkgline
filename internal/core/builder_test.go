package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"pkgline/internal/manifest"
	"pkgline/internal/path"
)

func TestCopyBuiltBinary(t *testing.T) {
	root := t.TempDir()
	name := "tool"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	src := filepath.Join(root, name)
	if err := os.WriteFile(src, []byte("bin"), 0755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), name)
	if err := copyBuiltBinary(root, name, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "bin" {
		t.Fatalf("copied = %q err=%v", got, err)
	}

	if err := copyBuiltBinary(t.TempDir(), "missing", dst); err == nil {
		t.Fatal("expected missing binary error")
	}
}

func TestBuildAndInstallRejectsInvalidManifest(t *testing.T) {
	m := &manifest.Manifest{}
	if _, err := BuildAndInstall(t.TempDir(), m, ""); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBuildAndInstallGo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(root, ".pkgline"))
	t.Setenv("PKGLINE_BIN", "")
	if err := path.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "go.mod"), []byte("module demo\n\ngo 1.20\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{}
	m.Package.Name = "demo"
	m.Package.Version = "0.1.0"
	m.Package.Language = "go"
	res, err := BuildAndInstall(pkg, m, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.InstallType != "native-go" {
		t.Fatalf("InstallType = %q", res.InstallType)
	}
	if _, err := os.Stat(res.ExecutablePath); err != nil {
		t.Fatal(err)
	}
}

func TestBuildAndInstallGoCmdLayout(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PKGLINE_ROOT", filepath.Join(root, ".pkgline"))
	t.Setenv("PKGLINE_BIN", "")
	if err := path.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(root, "pkg")
	cmdDir := filepath.Join(pkg, "cmd", "demo")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "go.mod"), []byte("module demo\n\ngo 1.20\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{}
	m.Package.Name = "demo"
	m.Package.Language = "go"
	res, err := BuildAndInstall(pkg, m, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.ExecutablePath); err != nil {
		t.Fatal(err)
	}
}

func TestScriptCommandUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	cmd := scriptCommand("/tmp/install.sh")
	if filepath.Base(cmd.Path) != "sh" && cmd.Args[0] != "sh" {
		t.Fatalf("unix script driver = %q %v", cmd.Path, cmd.Args)
	}
}

func TestBuildAndInstallUnsupportedLanguageWithoutScript(t *testing.T) {
	m := &manifest.Manifest{}
	m.Package.Name = "py"
	m.Package.Language = "python"
	if _, err := BuildAndInstall(t.TempDir(), m, ""); err == nil {
		t.Fatal("expected unsupported language error")
	}
}
