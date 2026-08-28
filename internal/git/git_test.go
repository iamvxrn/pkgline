package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneLocalDirectoryCopiesNestedFiles(t *testing.T) {
	src := t.TempDir()
	nested := filepath.Join(src, "sub")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "pkgline.toml"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := Clone(src, dst, ""); err != nil {
		t.Fatalf("Clone local: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "sub", "pkgline.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("copied content = %q", got)
	}
}

func TestCloneMissingPathFallsThroughToGitAndFails(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out")
	err := Clone(filepath.Join(t.TempDir(), "no-such-repo"), dst, "")
	if err == nil {
		t.Fatal("expected clone error")
	}
	if !strings.Contains(err.Error(), "git clone failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLooksLikeCommit(t *testing.T) {
	if !looksLikeCommit("abcdef0") || !looksLikeCommit(strings.Repeat("a", 40)) {
		t.Fatal("sha should match")
	}
	if looksLikeCommit("v1.0.0") || looksLikeCommit("main") || looksLikeCommit("abc") {
		t.Fatal("branch/tag should not look like a commit")
	}
}

func TestGitRepoHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	work := t.TempDir()
	runGit(t, work, "init")
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "Test")
	runGit(t, work, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", "README")
	runGit(t, work, "commit", "-m", "init")

	if !IsGitRepo(work) {
		t.Fatal("expected work tree to be a git repo")
	}
	if IsGitRepo(t.TempDir()) {
		t.Fatal("empty dir is not a git repo")
	}

	sha, err := GetHeadCommit(work)
	if err != nil {
		t.Fatal(err)
	}
	if len(sha) < 7 {
		t.Fatalf("short SHA: %q", sha)
	}

	if _, err := Pull(t.TempDir()); err == nil {
		t.Fatal("expected Pull on non-repo to fail")
	}
	if sha, err := GetHeadCommit(t.TempDir()); err == nil || sha != "" {
		t.Fatalf("failed rev-parse should return empty hash, got %q err %v", sha, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s (%v)", strings.Join(args, " "), out, err)
	}
}
