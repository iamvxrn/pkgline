package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"pkgline/internal/path"
)

type Config struct {
	BinDir  string            `toml:"bin_dir"`
	AppsDir string            `toml:"apps_dir"`
	Aliases map[string]string `toml:"aliases"`
}

func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := path.ConfigPath()

	defaultBinDir := filepath.Join(home, ".pkgline", "bin")
	defaultAppsDir := filepath.Join(home, ".pkgline", "apps")

	cfg := &Config{
		BinDir:  defaultBinDir,
		AppsDir: defaultAppsDir,
		Aliases: map[string]string{},
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	fileCfg := struct {
		BinDir  string            `toml:"bin_dir"`
		AppsDir string            `toml:"apps_dir"`
		Aliases map[string]string `toml:"aliases"`
	}{}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := toml.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if fileCfg.BinDir != "" {
		cfg.BinDir = fileCfg.BinDir
	}
	if fileCfg.AppsDir != "" {
		cfg.AppsDir = fileCfg.AppsDir
	}

	for alias, uri := range fileCfg.Aliases {
		cfg.Aliases[alias] = uri
	}

	return cfg, nil
}

// ResolveURI converts aliases or shorthand syntax (e.g. gh:owner/repo) into full Git URIs.
func (c *Config) ResolveURI(input string) string {
	input = strings.TrimSpace(input)
	if resolved, ok := c.Aliases[input]; ok {
		input = strings.TrimSpace(resolved)
	}

	// Handle gh:owner/repo format
	if strings.HasPrefix(input, "gh:") {
		repoPath := strings.TrimPrefix(input, "gh:")
		return fmt.Sprintf("https://github.com/%s.git", strings.Trim(repoPath, "/"))
	}

	// Handle owner/repo format if not local path or URL
	if !strings.Contains(input, "://") && !strings.HasPrefix(input, "git@") && !strings.HasPrefix(input, "/") && !strings.HasPrefix(input, ".") && strings.Count(input, "/") == 1 {
		return fmt.Sprintf("https://github.com/%s.git", input)
	}

	return input
}
