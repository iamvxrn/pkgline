package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"pkgline/internal/path"
)

// Key returns a stable cache key for a built binary.
// Inputs: original URI, git commit (or file hash for local), version, language, executable name, OS/arch,
// and the versions of the toolchains that build the selected language.
func Key(uri, commit, version, lang, execName string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s|%s", uri, commit, version, lang, execName, runtime.GOOS, runtime.GOARCH, goVersion(), toolchainVersion(lang))
	sum := fmt.Sprintf("%x", h.Sum(nil))
	return sum[:16]
}

func goVersion() string {
	return runtime.Version()
}

var toolchainVersionCommand = func(tool string) ([]byte, error) {
	return exec.Command(tool, "--version").Output()
}

func toolchainVersion(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	var tool string
	switch lang {
	case "rust":
		tool = "rustc"
	case "c", "cpp", "cbld":
		if lang == "cpp" {
			tool = os.Getenv("CXX")
		} else {
			tool = os.Getenv("CC")
		}
		if tool == "" {
			if lang == "cpp" {
				tool = "c++"
			} else {
				tool = "cc"
			}
		}
	}
	if tool == "" {
		return ""
	}

	out, err := toolchainVersionCommand(tool)
	if err != nil {
		return tool + ":unavailable"
	}
	return tool + ":" + strings.TrimSpace(string(out))
}

// Dir returns ~/.pkgline/cache/prebuilt
func Dir() string {
	return filepath.Join(path.CacheDir(), "prebuilt")
}

// Path returns the cached binary path for key+exec.
// Layout: <cache/prebuilt>/<key>/<exec>
func Path(key, execName string) string {
	return filepath.Join(Dir(), key, execName)
}

// Lookup checks if cached binary exists.
func Lookup(key, execName string) (string, bool) {
	p := Path(key, execName)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, true
	}
	return "", false
}

// Store copies binPath into the cache.
func Store(key, execName, binPath string) error {
	dst := Path(key, execName)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(binPath)
	if err != nil {
		return err
	}
	// Write atomically via temp + rename.
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Restore copies cached binary to binPath.
func Restore(key, execName, binPath string) error {
	src, ok := Lookup(key, execName)
	if !ok {
		return fmt.Errorf("cache miss for %s", key)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(binPath, data, 0755)
}
