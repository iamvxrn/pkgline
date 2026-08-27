package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pkgline/internal/config"
	"pkgline/internal/db"
	"pkgline/internal/git"
	"pkgline/internal/manifest"
	"pkgline/internal/path"
	"pkgline/internal/ui"
)

// Installer orchestrates package manager commands.
type Installer struct {
	cfg   *config.Config
	store *db.Store
}

// NewInstaller creates a new Installer instance.
func NewInstaller() (*Installer, error) {
	if err := path.EnsureDirs(); err != nil {
		return nil, err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	return &Installer{
		cfg:   cfg,
		store: db.NewStore(),
	}, nil
}

// Install fetches, compiles/executes, and records a new package.
func (ins *Installer) Install(rawURI string) error {
	resolvedURI := ins.cfg.ResolveURI(rawURI)
	ui.LogInfo("Installing from %s...", resolvedURI)

	// Stage clone in cache
	stagingDir := filepath.Join(path.CacheDir(), fmt.Sprintf("staging-%d", time.Now().UnixNano()))
	defer func() { _ = os.RemoveAll(stagingDir) }()

	if err := git.Clone(resolvedURI, stagingDir); err != nil {
		return fmt.Errorf("failed to fetch package repository: %w", err)
	}

	// Parse manifest
	m, err := manifest.LoadFromDir(stagingDir)
	if err != nil {
		return fmt.Errorf("failed to read package manifest: %w", err)
	}

	pkgName := m.Package.Name
	appDir := path.AppPath(pkgName)

	// Create binary backup before overwriting
	binName := m.GetExecutable()
	binPath := filepath.Join(path.BinDir(), binName)
	bakPath := binPath + ".bak"
	if _, err := os.Stat(binPath); err == nil {
		_ = copyFile(binPath, bakPath)
	}

	// If app already exists, remove existing directory for clean install
	if _, err := os.Stat(appDir); err == nil {
		ui.LogInfo("Overwriting existing installation of '%s'...", pkgName)
		_ = os.RemoveAll(appDir)
	}

	// Move staging to final app location
	if err := os.Rename(stagingDir, appDir); err != nil {
		// Fallback: directory copy if cross-device rename fails
		if copyErr := copyDir(stagingDir, appDir); copyErr != nil {
			return fmt.Errorf("failed to move package files to %s: %w", appDir, copyErr)
		}
	}

	// Perform smart installation build/script execution
	buildRes, err := BuildAndInstall(appDir, m)
	if err != nil {
		_ = os.RemoveAll(appDir) // Clean up target directory on build failure
		// Restore binary backup if available
		if _, statErr := os.Stat(bakPath); statErr == nil {
			_ = copyFile(bakPath, binPath)
		}
		return err
	}

	// Get Git commit
	commit, _ := git.GetHeadCommit(appDir)

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	rec := db.PackageRecord{
		Name:            pkgName,
		Version:         m.Package.Version,
		URI:             rawURI,
		Language:        m.GetLanguage(),
		Executable:      buildRes.ExecutableName,
		InstallType:     buildRes.InstallType,
		InstallScript:   m.Scripts.Install,
		UninstallScript: m.Scripts.Uninstall,
		AppDir:          appDir,
		BinPath:         buildRes.ExecutablePath,
		GitCommit:       commit,
		InstalledAt:     nowStr,
		UpdatedAt:       nowStr,
	}

	if err := ins.store.Set(rec); err != nil {
		return fmt.Errorf("failed to update inventory: %w", err)
	}

	ui.LogSuccess("Package '%s' v%s installed successfully!", pkgName, m.Package.Version)
	ui.LogInfo("Binary linked at: %s", buildRes.ExecutablePath)
	ui.CheckPathWarning()

	return nil
}

// Rollback restores a binary to its previous backup executable (.bak).
func (ins *Installer) Rollback(pkgName string) error {
	rec, exists, _ := ins.store.Get(pkgName)
	if !exists {
		return fmt.Errorf("package '%s' is not installed", pkgName)
	}

	binName := rec.Executable
	if binName == "" {
		binName = pkgName
	}

	binPath := filepath.Join(path.BinDir(), binName)
	bakPath := binPath + ".bak"

	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup executable found at %s", bakPath)
	}

	if err := copyFile(bakPath, binPath); err != nil {
		return fmt.Errorf("failed to restore backup binary: %w", err)
	}

	ui.LogSuccess("Rolled back binary for package '%s' from %s!", pkgName, bakPath)
	return nil
}

// Remove uninstalls a package and cleans up its files.
func (ins *Installer) Remove(pkgName string) error {
	rec, exists, _ := ins.store.Get(pkgName)
	appDir := path.AppPath(pkgName)

	if !exists {
		// Check if folder exists anyway
		if _, statErr := os.Stat(appDir); os.IsNotExist(statErr) {
			return fmt.Errorf("package '%s' is not installed", pkgName)
		}
	}

	ui.LogInfo("Removing package '%s'...", pkgName)

	// Try reading manifest for uninstall script hook
	m, err := manifest.LoadFromDir(appDir)
	if err == nil && m.Scripts.Uninstall != "" {
		ui.LogInfo("Executing uninstall script '%s'...", m.Scripts.Uninstall)
		if err := RunUninstallScript(appDir, m.Scripts.Uninstall, m); err != nil {
			ui.LogWarning("Uninstall script failed: %v", err)
		}
	}

	// Remove binary from ~/.pkgline/bin/
	binName := rec.Executable
	if binName == "" && m != nil {
		binName = m.GetExecutable()
	}
	if binName == "" {
		binName = pkgName
	}

	binPath := filepath.Join(path.BinDir(), binName)
	_ = os.Remove(binPath)
	_ = os.Remove(binPath + ".bak")

	// Remove app directory
	if err := os.RemoveAll(appDir); err != nil {
		ui.LogWarning("Failed to remove app directory %s: %v", appDir, err)
	}

	// Remove inventory record
	if err := ins.store.Remove(pkgName); err != nil {
		return fmt.Errorf("failed to update inventory: %w", err)
	}

	ui.LogSuccess("Package '%s' removed successfully.", pkgName)
	return nil
}

// Sync pulls updates from git for single package or all packages.
func (ins *Installer) Sync(targetPkg string) error {
	packages, err := ins.store.List()
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		ui.LogInfo("No packages installed to sync.")
		return nil
	}

	if targetPkg != "" {
		var filtered []db.PackageRecord
		for _, p := range packages {
			if p.Name == targetPkg {
				filtered = append(filtered, p)
				break
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("package '%s' is not installed", targetPkg)
		}
		packages = filtered
	}

	ui.LogInfo("Syncing %d package(s)...", len(packages))

	for _, rec := range packages {
		appDir := path.AppPath(rec.Name)
		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			ui.LogWarning("App directory for '%s' not found (%s). Skipping sync.", rec.Name, appDir)
			continue
		}

		ui.LogInfo("Syncing package '%s'...", rec.Name)
		pulled, err := git.Pull(appDir)
		if err != nil {
			ui.LogWarning("Failed to pull updates for '%s': %v. Continuing sync...", rec.Name, err)
			continue
		}

		m, err := manifest.LoadFromDir(appDir)
		if err != nil {
			ui.LogWarning("Failed to parse manifest for '%s' after pull: %v. Continuing sync...", rec.Name, err)
			continue
		}

		newCommit, _ := git.GetHeadCommit(appDir)
		versionChanged := m.Package.Version != rec.Version
		commitChanged := newCommit != rec.GitCommit

		if pulled || versionChanged || commitChanged {
			ui.LogInfo("Rebuilding '%s' (Version: %s -> %s, Commit: %s)...",
				rec.Name, rec.Version, m.Package.Version, shortenHash(newCommit))

			buildRes, err := BuildAndInstall(appDir, m)
			if err != nil {
				ui.LogError("Rebuild failed for '%s': %v. Continuing sync...", rec.Name, err)
				continue
			}

			// Update record
			rec.Version = m.Package.Version
			rec.GitCommit = newCommit
			rec.Executable = buildRes.ExecutableName
			rec.BinPath = buildRes.ExecutablePath
			rec.InstallType = buildRes.InstallType
			rec.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

			_ = ins.store.Set(rec)
			ui.LogSuccess("Package '%s' updated to v%s!", rec.Name, rec.Version)
		} else {
			ui.LogSuccess("Package '%s' is up to date (v%s).", rec.Name, rec.Version)
		}
	}

	return nil
}

// List prints installed package inventory.
func (ins *Installer) List(asJSON bool) error {
	packages, err := ins.store.List()
	if err != nil {
		return err
	}

	if asJSON {
		if packages == nil {
			packages = []db.PackageRecord{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(packages)
	}

	if len(packages) == 0 {
		ui.LogInfo("No packages installed. Use 'pkgline install <uri>' to install packages.")
		ui.CheckPathWarning()
		return nil
	}

	headers := []string{"Name", "Version", "Type", "Executable", "Installed At"}
	var rows [][]string

	for _, p := range packages {
		rows = append(rows, []string{
			p.Name,
			p.Version,
			p.InstallType,
			p.Executable,
			p.InstalledAt,
		})
	}

	ui.PrintTable(headers, rows)
	fmt.Println()
	ui.CheckPathWarning()

	return nil
}

func shortenHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
}
