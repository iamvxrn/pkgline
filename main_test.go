package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripJSONFlag(t *testing.T) {
	on, rest := stripJSONFlag([]string{"pkgline", "--json", "list", "extra"})
	if !on || len(rest) != 3 || rest[1] != "list" {
		t.Fatalf("got on=%v rest=%v", on, rest)
	}
	off, rest := stripJSONFlag([]string{"pkgline", "version"})
	if off || strings.Join(rest, " ") != "pkgline version" {
		t.Fatalf("got on=%v rest=%v", off, rest)
	}
}

func TestPrintUsage(t *testing.T) {
	out := captureStdout(t, printUsage)
	for _, want := range []string{"install", "doctor", "--json"} {
		if !strings.Contains(out, want) {
			t.Fatalf("usage missing %q:\n%s", want, out)
		}
	}
}

func TestPrintCompletion(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		out := captureStdout(t, func() { printCompletion(shell) })
		if !strings.Contains(strings.ToLower(out), "pkgline") {
			t.Fatalf("%s completion empty: %q", shell, out)
		}
	}
}

func TestRunDoctorJSON(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	t.Setenv("PKGLINE_ROOT", root)
	t.Setenv("PKGLINE_BIN", bin)
	t.Setenv("PATH", bin)

	out := captureStdout(t, func() { runDoctor(true) })
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if payload["version"] != version {
		t.Errorf("version = %v", payload["version"])
	}
	if payload["bin_in_path"] != true {
		t.Errorf("bin_in_path = %v", payload["bin_in_path"])
	}
}

func TestRunDoctorText(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PKGLINE_ROOT", root)
	t.Setenv("PKGLINE_BIN", filepath.Join(root, "bin"))
	t.Setenv("PATH", "/usr/bin")
	out := captureStdout(t, func() { runDoctor(false) })
	if !strings.Contains(out, "Pkgline Version") || !strings.Contains(out, "NOT in $PATH") {
		t.Fatalf("doctor text: %q", out)
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
