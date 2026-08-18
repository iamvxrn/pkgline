package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"pkgline/internal/manifest"
	"pkgline/internal/path"
	"pkgline/internal/ui"
)

// BuildResult contains metadata about the build process.
type BuildResult struct {
	InstallType     string
	ExecutablePath string
	ExecutableName string
}

// BuildAndInstall performs the smart installation logic based on the manifest.
func BuildAndInstall(appRoot string, m *manifest.Manifest) (*BuildResult, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid package manifest: %w", err)
	}

	execName := m.GetExecutable()
	binDir := path.BinDir()
	targetBinPath := filepath.Join(binDir, execName)
	lang := m.GetLanguage()

	switch lang {
	case "go":
		ui.LogInfo("Building Go native package '%s'...", m.Package.Name)
		if err := buildGoPackage(appRoot, targetBinPath); err != nil {
			return nil, fmt.Errorf("Go build failed: %w", err)
		}
		return &BuildResult{
			InstallType:     "native-go",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil
	default:
		return nil, fmt.Errorf("package '%s' specifies unsupported language '%s' (v0.1.0 supports go only)", m.Package.Name, lang)
	}
}

func buildGoPackage(appRoot, targetBinPath string) error {
	cmd := exec.Command("go", "build", "-o", targetBinPath, ".")
	cmd.Dir = appRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	return os.Chmod(targetBinPath, 0755)
}
