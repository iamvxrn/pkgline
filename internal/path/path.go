package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PkglineRoot returns the root directory for pkgline state (~/.pkgline by default).
// Can be overridden via PKGLINE_ROOT environment variable.
func PkglineRoot() string {
	if env := os.Getenv("PKGLINE_ROOT"); env != "" {
		return ExpandPath(env)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pkgline"
	}
	return filepath.Join(home, ".pkgline")
}

// BinDir returns ~/.pkgline/bin
func BinDir() string {
	return filepath.Join(PkglineRoot(), "bin")
}

// AppsDir returns ~/.pkgline/apps
func AppsDir() string {
	return filepath.Join(PkglineRoot(), "apps")
}

// AppPath returns ~/.pkgline/apps/<name>
func AppPath(name string) string {
	return filepath.Join(AppsDir(), name)
}

// CacheDir returns ~/.pkgline/cache
func CacheDir() string {
	return filepath.Join(PkglineRoot(), "cache")
}

// InventoryPath returns ~/.pkgline/inventory.json
func InventoryPath() string {
	return filepath.Join(PkglineRoot(), "inventory.json")
}

// ConfigDir returns ~/.config/pkgline (respects XDG_CONFIG_HOME if set)
func ConfigDir() string {
	if env := os.Getenv("PKGLINE_CONFIG_DIR"); env != "" {
		return ExpandPath(env)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(ExpandPath(xdg), "pkgline")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(PkglineRoot(), "config")
	}
	return filepath.Join(home, ".config", "pkgline")
}

// ConfigPath returns ~/.config/pkgline/config.toml
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// EnsureDirs creates necessary directory structures (~/.pkgline/bin, apps, cache, config).
func EnsureDirs() error {
	dirs := []string{
		BinDir(),
		AppsDir(),
		CacheDir(),
		ConfigDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// ExpandPath expands leading ~ to user's home directory.
func ExpandPath(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
