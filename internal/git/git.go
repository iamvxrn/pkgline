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
// ref is an optional branch, tag, or commit; empty means the remote default.
func Clone(uri string, targetDir string, ref string) error {
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

	args := []string{"clone", "--depth", "1", "--recurse-submodules", "--shallow-submodules"}
	if ref != "" && !looksLikeCommit(ref) {
		args = append(args, "--branch", ref)
	}
	// "--" stops git from reading a URI that begins with "-" as an option;
	// --upload-pack=<cmd> would otherwise run <cmd>.
	args = append(args, "--", uri, targetDir)
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed for %s: %s (%w)", uri, strings.TrimSpace(stderr.String()), err)
	}
	if ref != "" && looksLikeCommit(ref) {
		fetch := exec.Command("git", "fetch", "--depth", "1", "origin", ref)
		fetch.Dir = targetDir
		if out, err := fetch.CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch %s failed: %s (%w)", ref, strings.TrimSpace(string(out)), err)
		}
		co := exec.Command("git", "checkout", "--detach", ref)
		co.Dir = targetDir
		if out, err := co.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout %s failed: %s (%w)", ref, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func looksLikeCommit(ref string) bool {
	if len(ref) < 7 || len(ref) > 40 {
		return false
	}
	for _, c := range ref {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
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

func Checkout(repoDir, ref string) error {
	if ref == "" {
		return fmt.Errorf("empty git ref")
	}
	cmd := exec.Command("git", "checkout", "--detach", ref)
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git checkout %s failed in %s: %s (%w)", ref, repoDir, strings.TrimSpace(stderr.String()), err)
	}
	return nil
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
