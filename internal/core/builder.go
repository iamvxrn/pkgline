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

	case "rust":
		ui.LogInfo("Building Rust native package '%s'...", m.Package.Name)
		if err := buildRustPackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("Rust build failed: %w", err)
		}
		return &BuildResult{
			InstallType:     "native-rust",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	case "cbld", "c", "cpp":
		ui.LogInfo("Building Cbld C/C++ native package '%s'...", m.Package.Name)
		if err := buildCbldPackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("Cbld build failed: %w", err)
		}
		return &BuildResult{
			InstallType:     "native-cbld",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	default:
		// Fallback: Custom installation script
		installScript := strings.TrimSpace(m.Scripts.Install)
		if installScript == "" {
			return nil, fmt.Errorf("package '%s' specifies unsupported language '%s' and no install script", m.Package.Name, lang)
		}

		ui.LogInfo("Executing fallback install script '%s' for '%s'...", installScript, m.Package.Name)
		if err := runInstallScript(appRoot, installScript, m); err != nil {
			return nil, fmt.Errorf("install script execution failed: %w", err)
		}

		return &BuildResult{
			InstallType:     "script",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil
	}
}

// buildGoPackage runs `go build -o <targetBinPath>` in appRoot
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

// buildRustPackage runs `cargo build --release` in appRoot and moves binary from target/release/
func buildRustPackage(appRoot, execName, targetBinPath string) error {
	cmd := exec.Command("cargo", "build", "--release")
	cmd.Dir = appRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	releaseBin := filepath.Join(appRoot, "target", "release", execName)
	if _, err := os.Stat(releaseBin); os.IsNotExist(err) {
		return fmt.Errorf("expected compiled binary not found at %s", releaseBin)
	}

	data, err := os.ReadFile(releaseBin)
	if err != nil {
		return fmt.Errorf("failed to read generated Rust binary: %w", err)
	}

	if err := os.WriteFile(targetBinPath, data, 0755); err != nil {
		return fmt.Errorf("failed to copy Rust binary to %s: %w", targetBinPath, err)
	}

	return nil
}

// buildCbldPackage runs `cbld build` in appRoot and copies output binary to targetBinPath
func buildCbldPackage(appRoot, execName, targetBinPath string) error {
	cmd := exec.Command("cbld", "build")
	cmd.Dir = appRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cbld build execution error: %s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	possibleBins := []string{
		filepath.Join(appRoot, "target", "release", execName),
		filepath.Join(appRoot, "target", "debug", execName),
		filepath.Join(appRoot, execName),
	}

	var compiledBin string
	for _, path := range possibleBins {
		if _, err := os.Stat(path); err == nil {
			compiledBin = path
			break
		}
	}

	if compiledBin == "" {
		return fmt.Errorf("expected compiled Cbld binary '%s' not found after build", execName)
	}

	data, err := os.ReadFile(compiledBin)
	if err != nil {
		return fmt.Errorf("failed to read compiled Cbld binary: %w", err)
	}

	if err := os.WriteFile(targetBinPath, data, 0755); err != nil {
		return fmt.Errorf("failed to copy Cbld binary to %s: %w", targetBinPath, err)
	}

	return nil
}

// runInstallScript executes script.install with environment variables
func runInstallScript(appRoot, scriptName string, m *manifest.Manifest) error {
	scriptPath := filepath.Join(appRoot, scriptName)
	fi, err := os.Stat(scriptPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("install script '%s' does not exist in %s", scriptName, appRoot)
	}
	if err != nil {
		return fmt.Errorf("cannot access install script: %w", err)
	}

	// Ensure script is executable
	if fi.Mode()&0111 == 0 {
		_ = os.Chmod(scriptPath, fi.Mode()|0111)
	}

	binDir := path.BinDir()
	cmd := exec.Command("sh", scriptPath)
	cmd.Dir = appRoot

	// Prepare Environment
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("PKGLINE_BIN=%s", binDir),
		fmt.Sprintf("PKGLINE_APP_ROOT=%s", appRoot),
		fmt.Sprintf("PKGLINE_PACKAGE_NAME=%s", m.Package.Name),
		fmt.Sprintf("PKGLINE_PACKAGE_VERSION=%s", m.Package.Version),
		fmt.Sprintf("PKGLINE_EXECUTABLE=%s", m.GetExecutable()),
	)
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("script error: %s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	if stdout.Len() > 0 {
		ui.LogInfo("Script output:\n%s", strings.TrimSpace(stdout.String()))
	}

	return nil
}

// RunUninstallScript executes scripts.uninstall if present
func RunUninstallScript(appRoot, scriptName string, m *manifest.Manifest) error {
	scriptPath := filepath.Join(appRoot, scriptName)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		ui.LogWarning("Uninstall script '%s' specified in manifest but file does not exist, proceeding with clean removal", scriptName)
		return nil
	}

	cmd := exec.Command("sh", scriptPath)
	cmd.Dir = appRoot

	env := os.Environ()
	env = append(env,
		fmt.Sprintf("PKGLINE_BIN=%s", path.BinDir()),
		fmt.Sprintf("PKGLINE_APP_ROOT=%s", appRoot),
		fmt.Sprintf("PKGLINE_PACKAGE_NAME=%s", m.Package.Name),
		fmt.Sprintf("PKGLINE_PACKAGE_VERSION=%s", m.Package.Version),
		fmt.Sprintf("PKGLINE_EXECUTABLE=%s", m.GetExecutable()),
	)
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uninstall script failed: %s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	if stdout.Len() > 0 {
		ui.LogInfo("Uninstall script output:\n%s", strings.TrimSpace(stdout.String()))
	}

	return nil
}
