package core

import (
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
	defer os.RemoveAll(stagingDir)

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
