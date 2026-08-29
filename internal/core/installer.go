package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"pkgline/internal/cache"
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
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{cfg.BinDir, cfg.AppsDir, path.CacheDir(), path.ConfigDir()} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &Installer{
		cfg:   cfg,
		store: db.NewStore(),
	}, nil
}

// InstallOpts overrides inferred or manifest language and binary name.
type InstallOpts struct {
	Language   string
	Executable string
}

func (ins *Installer) appPath(name string) string {
	return filepath.Join(ins.cfg.AppsDir, name)
}

func (ins *Installer) binFile(name string) string {
	return filepath.Join(ins.cfg.BinDir, name)
}

// Install fetches, compiles/executes, and records a new package.
func (ins *Installer) Install(rawURI string) error {
	return ins.InstallWith(rawURI, InstallOpts{})
}

// InstallWith is Install plus CLI --lang / --exec overrides.
func (ins *Installer) InstallWith(rawURI string, opts InstallOpts) error {
	spec, ref := config.SplitRef(rawURI)
	resolvedURI := ins.cfg.ResolveURI(spec)
	ui.LogInfo("Installing from %s...", resolvedURI)
	if ref != "" {
		ui.LogInfo("Using ref %s", ref)
	}

	// Stage clone in cache
	stagingDir := filepath.Join(path.CacheDir(), fmt.Sprintf("staging-%d", time.Now().UnixNano()))
	defer func() { _ = os.RemoveAll(stagingDir) }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		_ = os.RemoveAll(stagingDir)
		os.Exit(130)
	}()

	if err := git.Clone(resolvedURI, stagingDir, ref); err != nil {
		return fmt.Errorf("failed to fetch package repository: %w", err)
	}

	// Parse manifest
	m, err := manifest.LoadFromDir(stagingDir)
	if err != nil {
		if strings.TrimSpace(opts.Language) == "" && strings.TrimSpace(opts.Executable) == "" {
			return fmt.Errorf("failed to read package manifest: %w", err)
		}
		m = &manifest.Manifest{
			Package: manifest.PackageConfig{
				Name:    nameFromURI(resolvedURI),
				Version: "0.0.0",
			},
		}
	}
	if err := m.ApplyOverrides(opts.Language, opts.Executable); err != nil {
		return fmt.Errorf("failed to apply install overrides: %w", err)
	}

	pkgName := m.Package.Name
	appDir := ins.appPath(pkgName)

	// Create binary backup before overwriting
	binName := m.GetExecutable()
	binPath := ins.binFile(binName)
	bakPath := binPath + ".bak"
	if _, err := os.Stat(binPath); err == nil {
		_ = copyFile(binPath, bakPath)
		if old, ok, _ := ins.store.Get(pkgName); ok {
			_ = writeRollbackMeta(binPath, old)
		}
	}

	// Cache key from commit in staging (pre-move)
	commitForCache, _ := git.GetHeadCommit(stagingDir)
	cacheKey := cache.Key(resolvedURI, commitForCache, m.Package.Version, m.GetLanguage(), binName)
	cachedPath, cacheHit := cache.Lookup(cacheKey, binName)

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

	var buildRes *BuildResult
	if cacheHit {
		ui.LogInfo("Cache hit for %s [%s] — restoring %s", pkgName, cacheKey[:7], cachedPath)
		if err := cache.Restore(cacheKey, binName, binPath); err != nil {
			ui.LogWarning("Cache restore failed, rebuilding: %v", err)
			cacheHit = false
		} else {
			buildRes = &BuildResult{
				InstallType:    "cached-" + m.GetLanguage(),
				ExecutablePath: binPath,
				ExecutableName: binName,
			}
			_ = os.Chmod(binPath, 0755)
		}
	}
	if !cacheHit {
		// Perform smart installation build/script execution
		var err error
		buildRes, err = BuildAndInstall(appDir, m, ins.cfg.BinDir)
		if err != nil {
			_ = os.RemoveAll(appDir) // Clean up target directory on build failure
			// Restore binary backup if available
			if _, statErr := os.Stat(bakPath); statErr == nil {
				_ = copyFile(bakPath, binPath)
			}
			return err
		}
		// Populate cache best-effort
		_ = cache.Store(cacheKey, binName, buildRes.ExecutablePath)
	}

	// Get Git commit (prefer cache commit, fallback to appDir)
	commit := commitForCache
	if commit == "" {
		commit, _ = git.GetHeadCommit(appDir)
	}

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

// Run fetches, builds to a temp bin and executes without persisting to inventory.
func (ins *Installer) Run(rawURI string, opts InstallOpts, binArgs []string) error {
	spec, ref := config.SplitRef(rawURI)
	resolvedURI := ins.cfg.ResolveURI(spec)
	ui.LogInfo("Running %s ...", resolvedURI)
	if ref != "" {
		ui.LogInfo("Using ref %s", ref)
	}

	stagingDir := filepath.Join(path.CacheDir(), fmt.Sprintf("run-staging-%d", time.Now().UnixNano()))
	tempBinDir := filepath.Join(os.TempDir(), fmt.Sprintf("pkgline-run-bin-%d", time.Now().UnixNano()))
	// Ensure cleanup.
	defer func() { _ = os.RemoveAll(stagingDir) }()
	defer func() { _ = os.RemoveAll(tempBinDir) }()
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp bin dir: %w", err)
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		_ = os.RemoveAll(stagingDir)
		_ = os.RemoveAll(tempBinDir)
		os.Exit(130)
	}()

	if err := git.Clone(resolvedURI, stagingDir, ref); err != nil {
		return fmt.Errorf("failed to fetch package repository: %w", err)
	}

	m, err := manifest.LoadFromDir(stagingDir)
	if err != nil {
		if strings.TrimSpace(opts.Language) == "" && strings.TrimSpace(opts.Executable) == "" {
			return fmt.Errorf("failed to read package manifest: %w", err)
		}
		m = &manifest.Manifest{
			Package: manifest.PackageConfig{
				Name:    nameFromURI(resolvedURI),
				Version: "0.0.0",
			},
		}
	}
	if err := m.ApplyOverrides(opts.Language, opts.Executable); err != nil {
		return fmt.Errorf("failed to apply run overrides: %w", err)
	}

	buildRes, err := BuildAndInstall(stagingDir, m, tempBinDir)
	if err != nil {
		return err
	}

	binPath := buildRes.ExecutablePath
	if _, err := os.Stat(binPath); err != nil {
		// Fallback: tempBinDir + execName
		binPath = filepath.Join(tempBinDir, m.GetExecutable())
		if _, err2 := os.Stat(binPath); err2 != nil {
			return fmt.Errorf("built binary not found at %s", binPath)
		}
	}

	ui.LogInfo("Executing %s %s", binPath, strings.Join(binArgs, " "))
	cmd := exec.Command(binPath, binArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Keep working dir as staging for packages that expect resources relative to source.
	cmd.Dir = stagingDir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(stagingDir)
		_ = os.RemoveAll(tempBinDir)
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
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

	binPath := ins.binFile(binName)
	bakPath := binPath + ".bak"

	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup executable found at %s", bakPath)
	}

	if err := copyFile(bakPath, binPath); err != nil {
		return fmt.Errorf("failed to restore backup binary: %w", err)
	}
	if err := ins.restoreRollbackMeta(binPath, pkgName); err != nil {
		ui.LogWarning("Restored binary but not inventory metadata: %v", err)
	}

	ui.LogSuccess("Rolled back package '%s' from %s!", pkgName, bakPath)
	return nil
}

// Remove uninstalls a package and cleans up its files.
func (ins *Installer) Remove(pkgName string) error {
	rec, exists, _ := ins.store.Get(pkgName)
	appDir := ins.appPath(pkgName)

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

	binPath := ins.binFile(binName)
	_ = os.Remove(binPath)
	_ = os.Remove(binPath + ".bak")
	_ = os.Remove(binPath + ".bak.json")

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

	// Fast path for single package: avoid goroutine overhead and keep logs ordered.
	if len(packages) == 1 {
		rec := packages[0]
		appDir := ins.appPath(rec.Name)
		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			ui.LogWarning("App directory for '%s' not found (%s). Skipping sync.", rec.Name, appDir)
			return nil
		}
		ui.LogInfo("Syncing package '%s'...", rec.Name)
		pulled := false
		if git.IsGitRepo(appDir) {
			var err error
			pulled, err = git.Pull(appDir)
			if err != nil {
				ui.LogWarning("Failed to pull updates for '%s': %v. Continuing sync...", rec.Name, err)
				return nil
			}
		}
		m, err := manifest.LoadFromDir(appDir)
		if err != nil {
			ui.LogWarning("Failed to parse manifest for '%s' after pull: %v. Continuing sync...", rec.Name, err)
			return nil
		}
		newCommit := rec.GitCommit
		if git.IsGitRepo(appDir) {
			if c, err := git.GetHeadCommit(appDir); err == nil {
				newCommit = c
			}
		}
		versionChanged := m.Package.Version != rec.Version
		commitChanged := newCommit != rec.GitCommit
		if pulled || versionChanged || commitChanged {
			ui.LogInfo("Rebuilding '%s' (Version: %s -> %s, Commit: %s)...", rec.Name, rec.Version, m.Package.Version, shortenHash(newCommit))
			binName := rec.Executable
			if binName == "" {
				binName = m.GetExecutable()
			}
			binPath := ins.binFile(binName)
			if _, err := os.Stat(binPath); err == nil {
				_ = copyFile(binPath, binPath+".bak")
				_ = writeRollbackMeta(binPath, rec)
			}
			// Try binary cache before rebuilding.
			cacheKey := cache.Key(rec.URI, newCommit, m.Package.Version, m.GetLanguage(), binName)
			if cached, ok := cache.Lookup(cacheKey, binName); ok {
				ui.LogInfo("Cache hit for %s [%s] — restoring %s", rec.Name, cacheKey[:7], cached)
				if err := cache.Restore(cacheKey, binName, binPath); err == nil {
					_ = os.Chmod(binPath, 0755)
					rec.Version = m.Package.Version
					rec.GitCommit = newCommit
					rec.Executable = binName
					rec.BinPath = binPath
					rec.InstallType = "cached-" + m.GetLanguage()
					rec.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
					_ = ins.store.Set(rec)
					ui.LogSuccess("Package '%s' updated to v%s! (from cache)", rec.Name, rec.Version)
					return nil
				}
				ui.LogWarning("Cache restore failed for %s, rebuilding", rec.Name)
			}
			buildRes, err := BuildAndInstall(appDir, m, ins.cfg.BinDir)
			if err != nil {
				ui.LogError("Rebuild failed for '%s': %v.", rec.Name, err)
				return nil
			}
			_ = cache.Store(cacheKey, binName, buildRes.ExecutablePath)
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
		return nil
	}

	// Parallel sync for multiple packages.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, r := range packages {
		rec := r
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			appDir := ins.appPath(rec.Name)
			if _, err := os.Stat(appDir); os.IsNotExist(err) {
				ui.LogWarning("App directory for '%s' not found (%s). Skipping sync.", rec.Name, appDir)
				return
			}
			ui.LogInfo("Syncing package '%s'...", rec.Name)
			pulled := false
			if git.IsGitRepo(appDir) {
				var err error
				pulled, err = git.Pull(appDir)
				if err != nil {
					ui.LogWarning("Failed to pull updates for '%s': %v. Continuing sync...", rec.Name, err)
					return
				}
			}
			m, err := manifest.LoadFromDir(appDir)
			if err != nil {
				ui.LogWarning("Failed to parse manifest for '%s' after pull: %v. Continuing sync...", rec.Name, err)
				return
			}
			newCommit := rec.GitCommit
			if git.IsGitRepo(appDir) {
				if c, err := git.GetHeadCommit(appDir); err == nil {
					newCommit = c
				}
			}
			versionChanged := m.Package.Version != rec.Version
			commitChanged := newCommit != rec.GitCommit
			if pulled || versionChanged || commitChanged {
				ui.LogInfo("Rebuilding '%s' (Version: %s -> %s, Commit: %s)...", rec.Name, rec.Version, m.Package.Version, shortenHash(newCommit))
				binName := rec.Executable
				if binName == "" {
					binName = m.GetExecutable()
				}
				binPath := ins.binFile(binName)
				if _, err := os.Stat(binPath); err == nil {
					_ = copyFile(binPath, binPath+".bak")
					_ = writeRollbackMeta(binPath, rec)
				}
				cacheKey := cache.Key(rec.URI, newCommit, m.Package.Version, m.GetLanguage(), binName)
				if cached, ok := cache.Lookup(cacheKey, binName); ok {
					ui.LogInfo("Cache hit for %s [%s] — restoring %s", rec.Name, cacheKey[:7], cached)
					if err := cache.Restore(cacheKey, binName, binPath); err == nil {
						_ = os.Chmod(binPath, 0755)
						rec.Version = m.Package.Version
						rec.GitCommit = newCommit
						rec.Executable = binName
						rec.BinPath = binPath
						rec.InstallType = "cached-" + m.GetLanguage()
						rec.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
						_ = ins.store.Set(rec)
						ui.LogSuccess("Package '%s' updated to v%s! (from cache)", rec.Name, rec.Version)
						return
					}
					ui.LogWarning("Cache restore failed for %s, rebuilding", rec.Name)
				}
				buildRes, err := BuildAndInstall(appDir, m, ins.cfg.BinDir)
				if err != nil {
					ui.LogError("Rebuild failed for '%s': %v. Continuing sync...", rec.Name, err)
					return
				}
				_ = cache.Store(cacheKey, binName, buildRes.ExecutablePath)
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
		}()
	}
	wg.Wait()

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

func nameFromURI(uri string) string {
	u := strings.TrimSuffix(strings.TrimRight(strings.ReplaceAll(uri, "\\", "/"), "/"), ".git")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	if u == "" || u == "." {
		return "package"
	}
	return u
}

func writeRollbackMeta(binPath string, rec db.PackageRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(binPath+".bak.json", data, 0644)
}

func (ins *Installer) restoreRollbackMeta(binPath, pkgName string) error {
	data, err := os.ReadFile(binPath + ".bak.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var rec db.PackageRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return err
	}
	if rec.Name == "" {
		rec.Name = pkgName
	}
	if rec.AppDir != "" && rec.GitCommit != "" && git.IsGitRepo(rec.AppDir) {
		if err := git.Checkout(rec.AppDir, rec.GitCommit); err != nil {
			ui.LogWarning("Could not check out previous commit: %v", err)
		}
	}
	return ins.store.Set(rec)
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
