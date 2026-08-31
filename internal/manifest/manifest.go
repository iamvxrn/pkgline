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
	MainPath   string `toml:"main_path"`
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
	case "go", "rust", "cbld", "c", "cpp", "make", "cmake", "zig", "node", "nodejs", "js", "ts":
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
		return fmt.Errorf("pkgline.toml: unsupported language '%s' (supported: go, rust, cbld, c, cpp, make, cmake, zig, node)", m.Package.Language)
	}

	if !hasNative && !hasInstallScript {
		return fmt.Errorf("pkgline.toml: package '%s' provides neither a supported language ('go', 'rust', 'cbld', 'c', 'cpp', 'make', 'cmake', 'zig', 'node') nor an install script", m.Package.Name)
	}

	if m.GetExecutable() == "" {
		return errors.New("pkgline.toml: package executable name could not be determined")
	}

	return nil
}

// ApplyOverrides sets CLI --lang / --exec on top of a loaded (or inferred)
// manifest, then re-validates. Empty strings leave the existing values.
func (m *Manifest) ApplyOverrides(language, executable string) error {
	if language = strings.TrimSpace(language); language != "" {
		m.Package.Language = language
	}
	if executable = strings.TrimSpace(executable); executable != "" {
		m.Package.Executable = executable
	}
	return m.Validate()
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
// If the file is missing, language is inferred from go.mod / Cargo.toml /
// cbld.toml / CMakeLists.txt / Makefile / install.sh.
// If language is unset in an existing manifest, infers make/cmake/go/rust/cbld the same way.
func LoadFromDir(dir string) (*Manifest, error) {
	target := filepath.Join(dir, ManifestFileName)
	if !fileExists(target) {
		m, err := inferManifest(dir)
		if err != nil {
			return nil, err
		}
		if err := m.Validate(); err != nil {
			return nil, err
		}
		return m, nil
	}
	m, err := parseFile(target)
	if err != nil {
		return nil, err
	}
	if m.GetLanguage() == "" && strings.TrimSpace(m.Scripts.Install) == "" {
		if lang := inferLanguage(dir); lang != "" {
			m.Package.Language = lang
		} else if fileExists(filepath.Join(dir, "install.sh")) {
			m.Scripts.Install = "install.sh"
		}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func inferLanguage(dir string) string {
	switch {
	case fileExists(filepath.Join(dir, "go.mod")):
		return "go"
	case fileExists(filepath.Join(dir, "Cargo.toml")):
		return "rust"
	case fileExists(filepath.Join(dir, "cbld.toml")):
		return "cbld"
	case fileExists(filepath.Join(dir, "build.zig")):
		return "zig"
	case fileExists(filepath.Join(dir, "package.json")):
		return "node"
	case fileExists(filepath.Join(dir, "CMakeLists.txt")):
		return "cmake"
	case fileExists(filepath.Join(dir, "Makefile")) || fileExists(filepath.Join(dir, "makefile")):
		return "make"
	default:
		return ""
	}
}

func inferManifest(dir string) (*Manifest, error) {
	name := filepath.Base(filepath.Clean(dir))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "package"
	}
	m := &Manifest{Package: PackageConfig{Name: name, Version: "0.0.0"}}
	if lang := inferLanguage(dir); lang != "" {
		m.Package.Language = lang
		if lang == "go" {
			if n := goModuleBaseName(dir); n != "" {
				m.Package.Name = n
			}
		}
		return m, nil
	}
	if fileExists(filepath.Join(dir, "install.sh")) {
		m.Scripts.Install = "install.sh"
		return m, nil
	}
	return nil, fmt.Errorf("cannot read manifest file %s: file does not exist (no go.mod, Cargo.toml, cbld.toml, CMakeLists.txt, Makefile, or install.sh to infer from)", targetPath(dir))
}

func targetPath(dir string) string {
	return filepath.Join(dir, ManifestFileName)
}

func goModuleBaseName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			mod := strings.TrimSpace(rest)
			if i := strings.LastIndex(mod, "/"); i >= 0 {
				return mod[i+1:]
			}
			return mod
		}
	}
	return ""
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
