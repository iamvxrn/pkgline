package pkglinefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReader_Plain(t *testing.T) {
	src := `
# Pkglinefile example
gh:user/repo1

gh:user/repo2@v1.0.0  # with version
  # blank and comment lines ignored

gh:user/repo3 --lang go --exec mybin
`
	f, err := ParseReader(strings.NewReader(src), "Pkglinefile")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(f.Entries) != 3 {
		t.Fatalf("want 3 entries, got %d: %+v", len(f.Entries), f.Entries)
	}
	if f.Entries[0].URI != "gh:user/repo1" {
		t.Fatalf("0 uri %q", f.Entries[0].URI)
	}
	if f.Entries[1].URI != "gh:user/repo2@v1.0.0" {
		t.Fatalf("1 uri %q", f.Entries[1].URI)
	}
	if f.Entries[2].URI != "gh:user/repo3" || f.Entries[2].Lang != "go" || f.Entries[2].Exec != "mybin" {
		t.Fatalf("2 entry %+v", f.Entries[2])
	}
}

func TestParseReader_InlineFlags(t *testing.T) {
	cases := []struct {
		line string
		uri  string
		lang string
		exec string
	}{
		{"gh:a/b --lang=rust", "gh:a/b", "rust", ""},
		{"gh:a/b --language go --exec=mybin", "gh:a/b", "go", "mybin"},
		{`gh:a/b --lang "go" --exec 'my bin'`, "gh:a/b", "go", "my bin"},
		{"gh:a/b --executable foo", "gh:a/b", "", "foo"},
	}
	for _, c := range cases {
		f, err := ParseReader(strings.NewReader(c.line), "Pkglinefile")
		if err != nil {
			t.Fatalf("line %q: %v", c.line, err)
		}
		if len(f.Entries) != 1 {
			t.Fatalf("line %q: want 1", c.line)
		}
		e := f.Entries[0]
		if e.URI != c.uri || e.Lang != c.lang || e.Exec != c.exec {
			t.Fatalf("line %q: got %+v want uri %q lang %q exec %q", c.line, e, c.uri, c.lang, c.exec)
		}
	}
}

func TestParseReader_UnknownFlag(t *testing.T) {
	_, err := ParseReader(strings.NewReader("gh:a/b --bogus"), "Pkglinefile")
	if err == nil {
		t.Fatal("want error for unknown flag")
	}
}

func TestParseReader_Comments(t *testing.T) {
	src := "gh:a/b # inline comment\n# full line\n\n gh:c/d \t # another\n"
	f, err := ParseReader(strings.NewReader(src), "Pkglinefile")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 2 || f.Entries[0].URI != "gh:a/b" || f.Entries[1].URI != "gh:c/d" {
		t.Fatalf("got %+v", f.Entries)
	}
	// quoted # should not be stripped
	src2 := "gh:a/b --exec \"my#bin\"\n"
	f, err = ParseReader(strings.NewReader(src2), "Pkglinefile")
	if err != nil {
		t.Fatal(err)
	}
	if f.Entries[0].Exec != "my#bin" {
		t.Fatalf("quoted # stripped: %+v", f.Entries[0])
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Pkglinefile")
	if err := os.WriteFile(path, []byte("gh:a/b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(sub)
	if err != nil || got != path {
		t.Fatalf("discover got %q err %v want %q", got, err, path)
	}
	// fallback name
	root2 := t.TempDir()
	sub2 := filepath.Join(root2, "x")
	_ = os.MkdirAll(sub2, 0755)
	fallback := filepath.Join(root2, "Pkglinefile.toml")
	_ = os.WriteFile(fallback, []byte("gh:a/b\n"), 0644)
	got, _ = Discover(sub2)
	if got != fallback {
		t.Fatalf("fallback discover %q want %q", got, fallback)
	}
	// not found
	empty := t.TempDir()
	got, err = Discover(empty)
	if err != nil || got != "" {
		t.Fatalf("not found: got %q err %v", got, err)
	}
}

func TestParseTomlFallback(t *testing.T) {
	toml := `
packages = ["gh:a/b", "gh:c/d --lang go"]
`
	f, err := ParseReader(strings.NewReader(toml), "Pkglinefile.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 2 {
		t.Fatalf("toml entries %d %+v", len(f.Entries), f.Entries)
	}
	if f.Entries[1].Lang != "go" {
		t.Fatalf("toml lang %+v", f.Entries[1])
	}
}

func TestParseTomlTables(t *testing.T) {
	toml := `
[[packages]]
uri = "gh:a/b"
lang = "rust"

[[packages]]
uri = "gh:c/d"
exec = "mybin"

[[packages]]
spec = "gh:e/f --lang go"
`
	f, err := ParseReader(strings.NewReader(toml), "Pkglinefile.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 3 {
		t.Fatalf("want 3 got %d %+v", len(f.Entries), f.Entries)
	}
	if f.Entries[0].URI != "gh:a/b" || f.Entries[0].Lang != "rust" {
		t.Fatalf("0 %+v", f.Entries[0])
	}
	if f.Entries[1].Exec != "mybin" {
		t.Fatalf("1 %+v", f.Entries[1])
	}
	if f.Entries[2].Lang != "go" || f.Entries[2].URI != "gh:e/f" {
		t.Fatalf("2 %+v", f.Entries[2])
	}
}

func TestResolvedURI(t *testing.T) {
	base := "/tmp/project"
	e := Entry{URI: "./local/pkg"}
	if got := e.ResolvedURI(base); got != "/tmp/project/local/pkg" {
		t.Fatalf("relative got %q", got)
	}
	e2 := Entry{URI: "gh:a/b"}
	if got := e2.ResolvedURI(base); got != "gh:a/b" {
		t.Fatalf("shorthand got %q", got)
	}
	e3 := Entry{URI: "https://github.com/a/b.git"}
	if got := e3.ResolvedURI(base); got != "https://github.com/a/b.git" {
		t.Fatalf("url got %q", got)
	}
}
