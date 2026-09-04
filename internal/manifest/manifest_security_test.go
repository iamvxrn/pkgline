package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A hostile pkgline.toml must not be able to steer the paths pkgline builds
// from it. name/executable are joined onto ~/.pkgline/apps and ~/.pkgline/bin.
func TestValidateRejectsTraversalNames(t *testing.T) {
	bad := []string{
		"../victim",
		"../../etc/cron.d/x",
		"a/b",
		`a\b`,
		"/etc/passwd",
		"..",
		".",
	}
	for _, name := range bad {
		t.Run("name="+name, func(t *testing.T) {
			m := &Manifest{Package: PackageConfig{Name: name, Language: "go"}}
			if err := m.Validate(); err == nil {
				t.Fatalf("Validate accepted traversing package name %q", name)
			}
		})
		t.Run("exec="+name, func(t *testing.T) {
			m := &Manifest{Package: PackageConfig{Name: "ok", Language: "go", Executable: name}}
			if err := m.Validate(); err == nil {
				t.Fatalf("Validate accepted traversing executable %q", name)
			}
		})
	}
}

func TestValidateRejectsEscapingScriptPaths(t *testing.T) {
	for _, script := range []string{"../../evil.sh", "/tmp/evil.sh", ".."} {
		m := &Manifest{Package: PackageConfig{Name: "ok"}, Scripts: ScriptConfig{Install: script}}
		if err := m.Validate(); err == nil {
			t.Fatalf("Validate accepted escaping install script %q", script)
		}
	}
	// A nested but contained script path stays legal.
	m := &Manifest{Package: PackageConfig{Name: "ok"}, Scripts: ScriptConfig{Install: "scripts/install.sh"}}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate rejected legitimate nested script path: %v", err)
	}
}

// ApplyOverrides is the --lang/--exec path; it must be validated too.
func TestApplyOverridesRejectsTraversal(t *testing.T) {
	m := &Manifest{Package: PackageConfig{Name: "ok", Language: "go"}}
	if err := m.ApplyOverrides("", "../../pwned"); err == nil {
		t.Fatal("ApplyOverrides accepted a traversing --exec value")
	}
}

// The manifests that pkgline's sibling projects actually ship must keep parsing.
func TestRealConsumerManifestsStayValid(t *testing.T) {
	consumers := map[string]string{
		"cbld": `[package]
name = "cbld"
version = "0.5.2"
language = "rust"
executable = "cbld"

[scripts]
install = "install.sh"
uninstall = "uninstall.sh"
`,
		"vibeporter": `[package]
name = "vibeporter"
version = "0.5.0"
language = "go"
executable = "vibeporter"

[scripts]
install = "install.sh"
uninstall = "uninstall.sh"
`,
	}
	for name, content := range consumers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, ManifestFileName)
			if err := os.WriteFile(p, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			m, err := LoadFromFile(p)
			if err != nil {
				t.Fatalf("consumer manifest %s no longer loads: %v", name, err)
			}
			if m.Package.Name != name {
				t.Errorf("name = %q, want %q", m.Package.Name, name)
			}
			if !m.IsNative() {
				t.Errorf("%s: expected a native language, got %q", name, m.GetLanguage())
			}
			// Documents the live contract gap: both ship an install hook that
			// the native build path never reaches.
			if strings.TrimSpace(m.Scripts.Install) == "" {
				t.Errorf("%s: expected an install script in the fixture", name)
			}
		})
	}
}
