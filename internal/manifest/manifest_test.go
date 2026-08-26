package manifest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr bool
	}{
		{
			name: "Valid Go Native",
			toml: `
[package]
name = "my-go-tool"
version = "0.1.0"
language = "go"
executable = "my-go-tool"
`,
			wantErr: false,
		},
		{
			name: "Valid Rust Native",
			toml: `
[package]
name = "my-rust-tool"
version = "0.1.0"
language = "rust"
`,
			wantErr: false,
		},
		{
			name: "Valid Cbld Native",
			toml: `
[package]
name = "my-cbld-c-tool"
version = "0.1.0"
language = "cbld"
`,
			wantErr: false,
		},
		{
			name: "Valid C Native",
			toml: `
[package]
name = "my-c-tool"
version = "0.1.0"
language = "c"
`,
			wantErr: false,
		},
		{
			name: "Valid Make Native",
			toml: `
[package]
name = "my-make-tool"
version = "0.1.0"
language = "make"
`,
			wantErr: false,
		},
		{
			name: "Valid CMake Native",
			toml: `
[package]
name = "my-cmake-tool"
version = "0.1.0"
language = "cmake"
`,
			wantErr: false,
		},
		{
			name: "Valid Script Fallback",
			toml: `
[package]
name = "script-tool"
version = "0.1.0"

[scripts]
install = "install.sh"
uninstall = "uninstall.sh"
`,
			wantErr: false,
		},
		{
			name: "Invalid No Native No Script",
			toml: `
[package]
name = "empty-tool"
version = "0.1.0"
`,
			wantErr: true,
		},
		{
			name: "Unsupported Language No Script",
			toml: `
[package]
name = "py-tool"
version = "0.1.0"
language = "python"
`,
			wantErr: true,
		},
		{
			name: "Missing Package Name",
			toml: `
[package]
version = "0.1.0"
language = "go"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			manifestPath := filepath.Join(tmpDir, "pkgline.toml")
			if err := os.WriteFile(manifestPath, []byte(tt.toml), 0644); err != nil {
				t.Fatalf("failed to write test manifest: %v", err)
			}

			m, err := LoadFromFile(manifestPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromFile() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				if m.Package.Name == "" {
					t.Errorf("Expected non-empty package name")
				}
			}
		})
	}
}

func TestDefaultExecutableName(t *testing.T) {
	m := &Manifest{
		Package: PackageConfig{
			Name: "auto-name",
		},
	}

	expectedDefault := "auto-name"
	if runtime.GOOS == "windows" {
		expectedDefault += ".exe"
	}

	if m.GetExecutable() != expectedDefault {
		t.Errorf("Expected default executable '%s', got '%s'", expectedDefault, m.GetExecutable())
	}

	m.Package.Executable = "custom-exec"
	expectedCustom := "custom-exec"
	if runtime.GOOS == "windows" {
		expectedCustom += ".exe"
	}
	if m.GetExecutable() != expectedCustom {
		t.Errorf("Expected custom executable '%s', got '%s'", expectedCustom, m.GetExecutable())
	}
}

func TestLoadFromDirInfersCMake(t *testing.T) {
	dir := t.TempDir()
	toml := `
[package]
name = "inferred"
version = "0.1.0"
executable = "inferred"
`
	if err := os.WriteFile(filepath.Join(dir, "pkgline.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("project(inferred)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if m.GetLanguage() != "cmake" {
		t.Errorf("expected cmake, got %q", m.GetLanguage())
	}
}

func TestLoadFromDirInfersMake(t *testing.T) {
	dir := t.TempDir()
	toml := `
[package]
name = "inferred-make"
version = "0.1.0"
executable = "inferred-make"
`
	if err := os.WriteFile(filepath.Join(dir, "pkgline.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n\ttrue\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if m.GetLanguage() != "make" {
		t.Errorf("expected make, got %q", m.GetLanguage())
	}
}
