package config

import (
	"fmt"
	"os"
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
	configPath := path.ConfigPath()

	cfg := &Config{
		BinDir:  path.BinDir(),
		AppsDir: path.AppsDir(),
		Aliases: map[string]string{
			"cbld":    "gh:iamvxrn/cbld",
			"muth":    "gh:iamvxrn/muth",
			"runa":    "gh:iamvxrn/runa",
			"pkgline": "gh:iamvxrn/pkgline",
		},
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

	// Env (PKGLINE_BIN / PKGLINE_APPS) wins over config.toml; file wins over defaults.
	if fileCfg.BinDir != "" && os.Getenv("PKGLINE_BIN") == "" {
		cfg.BinDir = path.ExpandPath(fileCfg.BinDir)
	}
	if fileCfg.AppsDir != "" && os.Getenv("PKGLINE_APPS") == "" {
		cfg.AppsDir = path.ExpandPath(fileCfg.AppsDir)
	}

	for alias, uri := range fileCfg.Aliases {
		cfg.Aliases[alias] = uri
	}

	return cfg, nil
}

// ResolveURI converts aliases or shorthand syntax into full Git URIs.
//
// Prefixes: gh: GitHub, gl: GitLab, cb: Codeberg, sh: Sourcehut.
// Bare owner/repo is GitHub. Full https:// and git@ URLs, and local paths, pass through.
func (c *Config) ResolveURI(input string) string {
	input = strings.TrimSpace(input)
	if resolved, ok := c.Aliases[input]; ok {
		input = strings.TrimSpace(resolved)
	}

	if rest, ok := strings.CutPrefix(input, "gh:"); ok {
		return fmt.Sprintf("https://github.com/%s.git", strings.Trim(rest, "/"))
	}
	if rest, ok := strings.CutPrefix(input, "gl:"); ok {
		return fmt.Sprintf("https://gitlab.com/%s.git", strings.Trim(rest, "/"))
	}
	if rest, ok := strings.CutPrefix(input, "cb:"); ok {
		return fmt.Sprintf("https://codeberg.org/%s.git", strings.Trim(rest, "/"))
	}
	if rest, ok := strings.CutPrefix(input, "sh:"); ok {
		rest = strings.Trim(rest, "/")
		if !strings.HasPrefix(rest, "~") {
			rest = "~" + rest
		}
		return fmt.Sprintf("https://git.sr.ht/%s.git", rest)
	}

	// Bare owner/repo → GitHub, unless it looks like a URL or local path.
	if !strings.Contains(input, "://") && !strings.HasPrefix(input, "git@") && !strings.HasPrefix(input, "/") && !strings.HasPrefix(input, ".") && strings.Count(input, "/") == 1 {
		return fmt.Sprintf("https://github.com/%s.git", input)
	}

	return input
}

// SplitRef peels an optional @tag, @branch, or @sha off a package spec.
// git@host:path is left intact; a second @ is treated as the ref.
func SplitRef(input string) (spec, ref string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}
	if strings.HasPrefix(input, "git@") {
		rest := input[len("git@"):]
		if i := strings.LastIndex(rest, "@"); i >= 0 {
			return "git@" + rest[:i], rest[i+1:]
		}
		return input, ""
	}
	if i := strings.LastIndex(input, "@"); i > 0 {
		return input[:i], input[i+1:]
	}
	return input, ""
}
