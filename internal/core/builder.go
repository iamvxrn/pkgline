package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"pkgline/internal/manifest"
	"pkgline/internal/path"
	"pkgline/internal/ui"
)

// BuildResult contains metadata about the build process.
type BuildResult struct {
	InstallType    string
	ExecutablePath string
	ExecutableName string
}

// BuildAndInstall performs the smart installation logic based on the manifest.
func BuildAndInstall(appRoot string, m *manifest.Manifest, binDir string) (*BuildResult, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid package manifest: %w", err)
	}

	// A manifest that declares BOTH a natively-supported language and a
	// [scripts] install hook only ever runs the native builder -- the script is
	// reachable only from the default branch below. Say so, rather than letting
	// the author believe their installer ran.
	if m.IsNative() && strings.TrimSpace(m.Scripts.Install) != "" {
		ui.LogWarning("Manifest for '%s' sets both language '%s' and [scripts] install = %q; the native builder is used and the install script is NOT run.",
			m.Package.Name, m.GetLanguage(), strings.TrimSpace(m.Scripts.Install))
	}

	execName := m.GetExecutable()
	if binDir == "" {
		binDir = path.BinDir()
	}
	targetBinPath := filepath.Join(binDir, execName)
	lang := m.GetLanguage()

	switch lang {
	case "go":
		ui.LogInfo("Building Go native package '%s'...", m.Package.Name)
		if err := buildGoPackage(appRoot, targetBinPath, m); err != nil {
			return nil, fmt.Errorf("go build failed: %w", err)
		}
		return &BuildResult{
			InstallType:    "native-go",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	case "rust":
		ui.LogInfo("Building Rust native package '%s'...", m.Package.Name)
		if err := buildRustPackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("rust build failed: %w", err)
		}
		return &BuildResult{
			InstallType:    "native-rust",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	case "cbld", "c", "cpp":
		ui.LogInfo("Building Cbld C/C++ native package '%s'...", m.Package.Name)
		if err := buildCbldPackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("cbld build failed: %w", err)
		}
		return &BuildResult{
			InstallType:    "native-cbld",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	case "make":
		ui.LogInfo("Building Make package '%s'...", m.Package.Name)
		if err := buildMakePackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("make build failed: %w", err)
		}
		return &BuildResult{
			InstallType:    "native-make",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	case "cmake":
		ui.LogInfo("Building CMake package '%s'...", m.Package.Name)
		if err := buildCMakePackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("cmake build failed: %w", err)
		}
		return &BuildResult{
			InstallType:    "native-cmake",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	case "zig":
		ui.LogInfo("Building Zig package '%s'...", m.Package.Name)
		if err := buildZigPackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("zig build failed: %w", err)
		}
		return &BuildResult{
			InstallType:    "native-zig",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil

	case "node", "nodejs", "js", "ts":
		ui.LogInfo("Building Node package '%s'...", m.Package.Name)
		if err := buildNodePackage(appRoot, execName, targetBinPath); err != nil {
			return nil, fmt.Errorf("node build failed: %w", err)
		}
		return &BuildResult{
			InstallType:    "native-node",
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
		if err := runInstallScript(appRoot, installScript, m, binDir); err != nil {
			return nil, fmt.Errorf("install script execution failed: %w", err)
		}

		return &BuildResult{
			InstallType:    "script",
			ExecutablePath: targetBinPath,
			ExecutableName: execName,
		}, nil
	}
}

// buildGoPackage runs `go build -o <targetBinPath>` in appRoot.
// Package.MainPath selects the package (default ".", then ./cmd/<name> if present).
func buildGoPackage(appRoot, targetBinPath string, m *manifest.Manifest) error {
	pkg := strings.TrimSpace(m.Package.MainPath)
	if pkg == "" {
		pkg = "."
		cmdDir := filepath.Join(appRoot, "cmd", m.Package.Name)
		if st, err := os.Stat(cmdDir); err == nil && st.IsDir() {
			if _, err := os.Stat(filepath.Join(appRoot, "main.go")); err != nil {
				pkg = "./cmd/" + m.Package.Name
			}
		}
	}
	cmd := exec.Command("go", "build", "-o", targetBinPath, pkg)
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

func buildMakePackage(appRoot, execName, targetBinPath string) error {
	cmd := exec.Command("make")
	cmd.Dir = appRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s (%w)", strings.TrimSpace(stderr.String()), err)
	}
	return copyBuiltBinary(appRoot, execName, targetBinPath)
}

func buildCMakePackage(appRoot, execName, targetBinPath string) error {
	configure := exec.Command("cmake", "-S", ".", "-B", "build", "-DCMAKE_BUILD_TYPE=Release")
	configure.Dir = appRoot
	var stdout, stderr bytes.Buffer
	configure.Stdout = &stdout
	configure.Stderr = &stderr
	if err := configure.Run(); err != nil {
		return fmt.Errorf("cmake configure: %s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	build := exec.Command("cmake", "--build", "build", "--config", "Release")
	build.Dir = appRoot
	stdout.Reset()
	stderr.Reset()
	build.Stdout = &stdout
	build.Stderr = &stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("cmake build: %s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	buildDir := filepath.Join(appRoot, "build")
	if err := copyBuiltBinary(buildDir, execName, targetBinPath); err == nil {
		return nil
	}
	return copyBuiltBinary(appRoot, execName, targetBinPath)
}

func buildZigPackage(appRoot, execName, targetBinPath string) error {
	cmd := exec.Command("zig", "build")
	cmd.Dir = appRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s (%w)", strings.TrimSpace(stderr.String()), err)
	}
	candidates := []string{
		filepath.Join(appRoot, "zig-out", "bin", execName),
		filepath.Join(appRoot, "zig-out", "bin", execName+".exe"),
		filepath.Join(appRoot, execName),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("failed to read zig binary: %w", err)
			}
			return os.WriteFile(targetBinPath, data, 0755)
		}
	}
	return fmt.Errorf("zig build succeeded but binary '%s' not found under zig-out/bin", execName)
}

func buildNodePackage(appRoot, execName, targetBinPath string) error {
	pkgJSONPath := filepath.Join(appRoot, "package.json")
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return fmt.Errorf("package.json not found: %w", err)
	}
	var pkg struct {
		Bin     interface{}       `json:"bin"`
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Install deps: prefer npm ci if lock exists, else npm install
	installCmd := "install"
	if _, err := os.Stat(filepath.Join(appRoot, "package-lock.json")); err == nil {
		installCmd = "ci"
	}
	cmd := exec.Command("npm", installCmd)
	cmd.Dir = appRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm %s: %s (%w)", installCmd, strings.TrimSpace(stderr.String()), err)
	}

	// If there's a build script, run it best-effort
	if _, ok := pkg.Scripts["build"]; ok {
		bcmd := exec.Command("npm", "run", "build")
		bcmd.Dir = appRoot
		stdout.Reset()
		stderr.Reset()
		bcmd.Stdout = &stdout
		bcmd.Stderr = &stderr
		_ = bcmd.Run()
	}

	// Resolve bin path from package.json
	var binRel string
	switch v := pkg.Bin.(type) {
	case string:
		binRel = v
	case map[string]interface{}:
		if s, ok := v[execName].(string); ok {
			binRel = s
		} else {
			// pick first bin entry
			for _, val := range v {
				if s, ok := val.(string); ok {
					binRel = s
					break
				}
			}
		}
	}
	candidates := []string{}
	if binRel != "" {
		candidates = append(candidates, filepath.Join(appRoot, binRel))
	}
	candidates = append(candidates,
		filepath.Join(appRoot, "dist", "index.js"),
		filepath.Join(appRoot, "dist", execName+".js"),
		filepath.Join(appRoot, "bin", execName+".js"),
		filepath.Join(appRoot, execName+".js"),
		filepath.Join(appRoot, "index.js"),
		filepath.Join(appRoot, binRel),
	)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			// For JS, ensure shebang or wrap; just copy and chmod
			binData, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("failed to read node bin: %w", err)
			}
			// If file lacks shebang, prepend node shebang
			if !strings.HasPrefix(string(binData), "#!") {
				binData = append([]byte("#!/usr/bin/env node\n"), binData...)
			}
			return os.WriteFile(targetBinPath, binData, 0755)
		}
	}
	return fmt.Errorf("node build: bin '%s' not found (checked package.json bin, dist/, bin/)", execName)
}

func copyBuiltBinary(searchRoot, execName, targetBinPath string) error {
	candidates := []string{
		filepath.Join(searchRoot, execName),
		filepath.Join(searchRoot, "Release", execName),
		filepath.Join(searchRoot, "release", execName),
	}
	var src string
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			src = p
			break
		}
	}
	if src == "" {
		return fmt.Errorf("expected compiled binary '%s' not found under %s", execName, searchRoot)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read compiled binary: %w", err)
	}
	return os.WriteFile(targetBinPath, data, 0755)
}

// runInstallScript executes script.install with environment variables
func runInstallScript(appRoot, scriptName string, m *manifest.Manifest, binDir string) error {
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

	if binDir == "" {
		binDir = path.BinDir()
	}
	cmd := scriptCommand(scriptPath)
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

	cmd := scriptCommand(scriptPath)
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

	return nil
}

func scriptCommand(scriptPath string) *exec.Cmd {
	ext := strings.ToLower(filepath.Ext(scriptPath))
	if runtime.GOOS == "windows" {
		switch ext {
		case ".ps1":
			return exec.Command("powershell", "-NoProfile", "-File", scriptPath)
		case ".bat", ".cmd":
			return exec.Command("cmd", "/C", scriptPath)
		}
		if _, err := exec.LookPath("bash"); err == nil {
			return exec.Command("bash", scriptPath)
		}
	}
	return exec.Command("sh", scriptPath)
}
