package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyStable(t *testing.T) {
	k1 := Key("gh:a/b", "abc123", "0.1.0", "go", "mybin")
	k2 := Key("gh:a/b", "abc123", "0.1.0", "go", "mybin")
	if k1 != k2 || len(k1) != 16 {
		t.Fatalf("key %q %q", k1, k2)
	}
	k3 := Key("gh:a/b", "def456", "0.1.0", "go", "mybin")
	if k1 == k3 {
		t.Fatal("different commit should differ")
	}
	k4 := Key("gh:a/b", "abc123", "0.2.0", "go", "mybin")
	if k1 == k4 {
		t.Fatal("different version should differ")
	}
}

func TestKeyIncludesRelevantToolchain(t *testing.T) {
	original := toolchainVersionCommand
	t.Cleanup(func() { toolchainVersionCommand = original })
	toolchain := "1.2.3"
	toolchainVersionCommand = func(tool string) ([]byte, error) {
		return []byte(tool + " " + toolchain), nil
	}
	t.Setenv("CC", "cc")
	if got := toolchainVersion("rust"); got != "rustc:rustc 1.2.3" {
		t.Fatalf("rust toolchain version = %q", got)
	}
	if got := toolchainVersion("c"); got != "cc:cc 1.2.3" {
		t.Fatalf("C toolchain version = %q", got)
	}
	if got := toolchainVersion("go"); got != "" {
		t.Fatalf("Go should use the existing runtime version field, got %q", got)
	}

	key := Key("u", "c", "v", "rust", "bin")
	toolchain = "2.0.0"
	if key == Key("u", "c", "v", "rust", "bin") {
		t.Fatal("toolchain version should affect key")
	}
}

func TestStoreLookupRestore(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PKGLINE_ROOT", root)

	// create a fake binary
	bin := filepath.Join(root, "mybin")
	if err := os.WriteFile(bin, []byte("binarydata"), 0755); err != nil {
		t.Fatal(err)
	}
	key := Key("gh:a/b", "commit123", "0.1.0", "go", "mybin")
	if err := Store(key, "mybin", bin); err != nil {
		t.Fatalf("store: %v", err)
	}
	cached, ok := Lookup(key, "mybin")
	if !ok {
		t.Fatal("lookup miss")
	}
	if cached != Path(key, "mybin") {
		t.Fatalf("path %q != %q", cached, Path(key, "mybin"))
	}
	// restore to new location
	dst := filepath.Join(root, "bin", "mybin")
	if err := Restore(key, "mybin", dst); err != nil {
		t.Fatalf("restore: %v", err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "binarydata" {
		t.Fatalf("data %q", string(data))
	}
	// miss
	if _, ok := Lookup("missing", "mybin"); ok {
		t.Fatal("should miss")
	}
}
