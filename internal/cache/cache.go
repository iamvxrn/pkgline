package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"pkgline/internal/path"
)

// Key returns a stable cache key for a built binary.
// Inputs: original URI, git commit (or file hash for local), version, language, executable name, OS/arch.
func Key(uri, commit, version, lang, execName string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s", uri, commit, version, lang, execName, runtime.GOOS, runtime.GOARCH, goVersion())
	sum := fmt.Sprintf("%x", h.Sum(nil))
	return sum[:16]
}

func goVersion() string {
	return runtime.Version()
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
