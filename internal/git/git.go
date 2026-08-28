package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Clone clones a git repository or copies a local directory into targetDir.
func Clone(uri string, targetDir string) error {
	// If URI is a local directory, handle accordingly
	fi, err := os.Stat(uri)
	if err == nil && fi.IsDir() {
		absPath, err := filepath.Abs(uri)
		if err != nil {
			absPath = uri
		}
		// Copy directory directly for local path to ensure uncommitted files (like pkgline.toml) are present
		if copyErr := copyDir(absPath, targetDir); copyErr != nil {
			return fmt.Errorf("local directory copy failed for %s: %w", absPath, copyErr)
		}
		return nil
	}

	// Normal Git remote URL clone
	cmd := exec.Command("git", "clone", "--depth", "1", "--recurse-submodules", "--shallow-submodules", uri, targetDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed for %s: %s (%w)", uri, strings.TrimSpace(stderr.String()), err)
	}

	return nil
}

// Pull runs `git pull` inside repoDir and reports if updates were downloaded.
func Pull(repoDir string) (bool, error) {
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = repoDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("git pull failed in %s: %s (%w)", repoDir, strings.TrimSpace(stderr.String()), err)
	}

	out := stdout.String()
	alreadyUpToDate := strings.Contains(out, "Already up to date") || strings.Contains(out, "Already up-to-date")
	return !alreadyUpToDate, nil
}

// GetHeadCommit returns the current HEAD commit hash.
func GetHeadCommit(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %s (%w)", strings.TrimSpace(stderr.String()), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// IsGitRepo checks whether dir is inside a git repository.
func IsGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// copyDir recursively copies src directory to dst.
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
