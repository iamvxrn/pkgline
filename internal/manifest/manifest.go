package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// ManifestFileName is the default manifest filename
const ManifestFileName = "pkgline.toml"

// PackageConfig holds package metadata in pkgline.toml
type PackageConfig struct {
	Name       string `toml:"name"`
	Version    string `toml:"version"`
	Language   string `toml:"language"`
	Executable string `toml:"executable"`
}

// ScriptConfig holds custom script hooks in pkgline.toml
type ScriptConfig struct {
	Install   string `toml:"install"`
	Uninstall string `toml:"uninstall"`
}

// Manifest represents the structure of pkgline.toml
type Manifest struct {
	Package PackageConfig `toml:"package"`
	Scripts ScriptConfig  `toml:"scripts"`
}

// GetExecutable returns the configured executable name (with .exe on Windows) or defaults to package name.
func (m *Manifest) GetExecutable() string {
	execName := strings.TrimSpace(m.Package.Executable)
	if execName == "" {
		execName = strings.TrimSpace(m.Package.Name)
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(execName), ".exe") {
		execName += ".exe"
	}
	return execName
}

// GetLanguage returns normalized language (lowercase).
func (m *Manifest) GetLanguage() string {
	return strings.ToLower(strings.TrimSpace(m.Package.Language))
}

// IsNative returns true if language is a supported native builder.
func (m *Manifest) IsNative() bool {
	lang := m.GetLanguage()
	switch lang {
	case "go", "rust", "cbld", "c", "cpp", "make", "cmake":
		return true
	default:
		return false
	}
}

// Validate ensures manifest contains valid metadata and install instructions.
func (m *Manifest) Validate() error {
	if strings.TrimSpace(m.Package.Name) == "" {
		return errors.New("pkgline.toml: [package] name is required")
	}

	lang := m.GetLanguage()
	hasNative := m.IsNative()
	hasInstallScript := strings.TrimSpace(m.Scripts.Install) != ""

	if lang != "" && !hasNative {
		return fmt.Errorf("pkgline.toml: unsupported language '%s' (supported: go, rust, cbld, c, cpp, make, cmake)", m.Package.Language)
	}

	if !hasNative && !hasInstallScript {
		return fmt.Errorf("pkgline.toml: package '%s' provides neither a supported language ('go', 'rust', 'cbld', 'c', 'cpp', 'make', 'cmake') nor an install script", m.Package.Name)
	}

	if m.GetExecutable() == "" {
		return errors.New("pkgline.toml: package executable name could not be determined")
	}

	return nil
}

// LoadFromFile parses a pkgline.toml file at the given path.
func LoadFromFile(filePath string) (*Manifest, error) {
	m, err := parseFile(filePath)
	if err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadFromDir looks for pkgline.toml in directory and parses it.
// If language is unset, infers make/cmake from Makefile or CMakeLists.txt.
func LoadFromDir(dir string) (*Manifest, error) {
	target := filepath.Join(dir, ManifestFileName)
	m, err := parseFile(target)
	if err != nil {
		return nil, err
	}
	if m.GetLanguage() == "" && strings.TrimSpace(m.Scripts.Install) == "" {
		if fileExists(filepath.Join(dir, "CMakeLists.txt")) {
			m.Package.Language = "cmake"
		} else if fileExists(filepath.Join(dir, "Makefile")) || fileExists(filepath.Join(dir, "makefile")) {
			m.Package.Language = "make"
		}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func parseFile(filePath string) (*Manifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read manifest file %s: %w", filePath, err)
	}
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest TOML %s: %w", filePath, err)
	}
	return &m, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
