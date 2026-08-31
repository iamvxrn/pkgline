package ui

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestColorizeHonorsUseColor(t *testing.T) {
	useColor = false
	if got := colorize(colorRed, "x"); got != "x" {
		t.Fatalf("disabled color: %q", got)
	}
	useColor = true
	got := colorize(colorRed, "x")
	if !strings.Contains(got, colorRed) || !strings.HasSuffix(got, colorReset) {
		t.Fatalf("enabled color: %q", got)
	}
	useColor = false
}

func TestLogWriters(t *testing.T) {
	useColor = false
	out := captureStdout(t, func() {
		LogInfo("hello %s", "info")
		LogSuccess("ok")
		LogWarning("careful")
	})
	if !strings.Contains(out, "hello info") || !strings.Contains(out, "✔") || !strings.Contains(out, "⚠") {
		t.Fatalf("stdout logs: %q", out)
	}

	errOut := captureStderr(t, func() {
		LogError("boom %d", 1)
	})
	if !strings.Contains(errOut, "boom 1") {
		t.Fatalf("stderr: %q", errOut)
	}
}

func TestCheckPathWarning(t *testing.T) {
	useColor = false
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	t.Setenv("PKGLINE_ROOT", root)
	t.Setenv("PKGLINE_BIN", bin)

	t.Setenv("PATH", "/usr/bin")
	missing := captureStdout(t, CheckPathWarning)
	if !strings.Contains(missing, "not in your PATH") {
		t.Fatalf("expected PATH warning, got %q", missing)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin")
	present := captureStdout(t, CheckPathWarning)
	if strings.Contains(present, "not in your PATH") {
		t.Fatalf("unexpected warning: %q", present)
	}
}

func TestPrintTable(t *testing.T) {
	useColor = false
	out := captureStdout(t, func() {
		PrintTable([]string{"Name", "Ver"}, [][]string{{"pkg", "1.0"}})
	})
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "pkg") {
		t.Fatalf("table: %q", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}
