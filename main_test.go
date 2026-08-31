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

func TestIsHelpArg(t *testing.T) {
	for _, a := range []string{"--help", "-h", "help", "HELP"} {
		if !isHelpArg(a) {
			t.Fatalf("expected help: %q", a)
		}
	}
	if isHelpArg("gh:user/repo") || isHelpArg("") {
		t.Fatal("uri must not look like help")
	}
}

func TestParseBootstrapArgs(t *testing.T) {
	f, dry, yes, err := parseBootstrapArgs([]string{"--file", "Pkglinefile", "--dry-run"})
	if err != nil || f != "Pkglinefile" || !dry || yes {
		t.Fatalf("got f=%q dry=%v yes=%v err=%v", f, dry, yes, err)
	}
	f, dry, yes, err = parseBootstrapArgs([]string{"--file=./Pkglinefile.toml", "--yes"})
	if err != nil || f != "./Pkglinefile.toml" || dry || !yes {
		t.Fatalf("file=: %q dry=%v yes=%v err=%v", f, dry, yes, err)
	}
	f, dry, yes, err = parseBootstrapArgs(nil)
	if err != nil || f != "" || dry || yes {
		t.Fatalf("empty: %q dry=%v yes=%v err=%v", f, dry, yes, err)
	}
	if _, _, _, err := parseBootstrapArgs([]string{"--bogus"}); err == nil {
		t.Fatal("want unknown flag")
	}
	if _, _, _, err := parseBootstrapArgs([]string{"--file"}); err == nil {
		t.Fatal("want missing value")
	}
	if _, _, _, err := parseBootstrapArgs([]string{"extra"}); err == nil {
		t.Fatal("want unexpected arg")
	}
}

func TestParseSearchArgs(t *testing.T) {
	q, n, err := parseSearchArgs([]string{"--limit", "5", "json", "parser"})
	if err != nil || q != "json parser" || n != 5 {
		t.Fatalf("got q=%q n=%d err=%v", q, n, err)
	}
	q, n, err = parseSearchArgs([]string{"--limit=2", "a"})
	if err != nil || q != "a" || n != 2 {
		t.Fatalf("limit=: %q %d %v", q, n, err)
	}
	q, n, err = parseSearchArgs([]string{"hello"})
	if err != nil || q != "hello" || n != 10 {
		t.Fatalf("default: %q %d %v", q, n, err)
	}
	if _, _, err := parseSearchArgs(nil); err == nil {
		t.Fatal("want missing query")
	}
	if _, _, err := parseSearchArgs([]string{"--limit", "bad", "x"}); err == nil {
		t.Fatal("want bad limit")
	}
	if _, _, err := parseSearchArgs([]string{"--bogus", "x"}); err == nil {
		t.Fatal("want unknown flag")
	}
}

func TestParseInstallArgs(t *testing.T) {
	uri, lang, execName, err := parseInstallArgs([]string{"--lang", "go", "--exec=mybin", "gh:user/repo"})
	if err != nil || uri != "gh:user/repo" || lang != "go" || execName != "mybin" {
		t.Fatalf("got uri=%q lang=%q exec=%q err=%v", uri, lang, execName, err)
	}
	uri, lang, execName, err = parseInstallArgs([]string{"gh:user/repo", "--language=rust"})
	if err != nil || uri != "gh:user/repo" || lang != "rust" || execName != "" {
		t.Fatalf("trailing flag: uri=%q lang=%q exec=%q err=%v", uri, lang, execName, err)
	}
	if _, _, _, err := parseInstallArgs([]string{"--lang"}); err == nil {
		t.Fatal("expected missing value")
	}
	if _, _, _, err := parseInstallArgs([]string{"--bogus", "gh:x/y"}); err == nil {
		t.Fatal("expected unknown flag")
	}
	if _, _, _, err := parseInstallArgs([]string{"a", "b"}); err == nil {
		t.Fatal("expected extra argument")
	}
	if _, _, _, err := parseInstallArgs(nil); err == nil {
		t.Fatal("expected missing uri")
	}
}

func TestStripJSONFlag(t *testing.T) {
	on, rest := stripJSONFlag([]string{"pkgline", "--json", "list", "extra"})
	if !on || len(rest) != 3 || rest[1] != "list" {
		t.Fatalf("got on=%v rest=%v", on, rest)
	}
	off, rest := stripJSONFlag([]string{"pkgline", "version"})
	if off || strings.Join(rest, " ") != "pkgline version" {
		t.Fatalf("got on=%v rest=%v", off, rest)
	}
	on, rest = stripJSONFlag([]string{"pkgline", "run", "gh:a/b", "--", "--json", "value"})
	if on || strings.Join(rest, " ") != "pkgline run gh:a/b -- --json value" {
		t.Fatalf("binary args were modified: on=%v rest=%v", on, rest)
	}
}

func TestParseRunArgsForwardsArgumentsAfterSeparator(t *testing.T) {
	uri, lang, execName, binArgs, err := parseRunArgs([]string{"--lang", "rust", "gh:user/repo", "--", "--name", "value", "-x"})
	if err != nil {
		t.Fatal(err)
	}
	if uri != "gh:user/repo" || lang != "rust" || execName != "" {
		t.Fatalf("got uri=%q lang=%q exec=%q", uri, lang, execName)
	}
	want := []string{"--name", "value", "-x"}
	if strings.Join(binArgs, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("bin args = %v, want %v", binArgs, want)
	}
}

func TestPrintUsage(t *testing.T) {
	out := captureStdout(t, printUsage)
	for _, want := range []string{"install", "bootstrap", "search", "doctor", "--json", "--lang", "--exec"} {
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
	if !strings.Contains(out, "Pkgline") || !strings.Contains(out, "Version") || !strings.Contains(out, "NOT in $PATH") {
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
