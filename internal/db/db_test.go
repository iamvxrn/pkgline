package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	store := NewStoreWithPath(filepath.Join(t.TempDir(), "inventory.json"))

	inv, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Packages) != 0 {
		t.Fatalf("empty inventory should have no packages, got %d", len(inv.Packages))
	}

	a := PackageRecord{Name: "zeta", Version: "1.0", InstallType: "native-go"}
	b := PackageRecord{Name: "alpha", Version: "2.0", InstallType: "script"}
	if err := store.Set(a); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(b); err != nil {
		t.Fatal(err)
	}

	got, ok, err := store.Get("alpha")
	if err != nil || !ok || got.Version != "2.0" {
		t.Fatalf("Get alpha = %+v ok=%v err=%v", got, ok, err)
	}
	_, ok, err = store.Get("missing")
	if err != nil || ok {
		t.Fatalf("Get missing: ok=%v err=%v", ok, err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "alpha" || list[1].Name != "zeta" {
		t.Fatalf("List not sorted: %+v", list)
	}

	if err := store.Remove("alpha"); err != nil {
		t.Fatal(err)
	}
	_, ok, err = store.Get("alpha")
	if err != nil || ok {
		t.Fatalf("alpha still present after Remove")
	}
}

func TestLoadEmptyAndInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatal(err)
	}
	inv, err := NewStoreWithPath(empty).Load()
	if err != nil || len(inv.Packages) != 0 {
		t.Fatalf("empty file: inv=%+v err=%v", inv, err)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStoreWithPath(bad).Load(); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestSaveNilPackagesMap(t *testing.T) {
	store := NewStoreWithPath(filepath.Join(t.TempDir(), "nested", "inventory.json"))
	if err := store.Save(&Inventory{}); err != nil {
		t.Fatal(err)
	}
	inv, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if inv.Packages == nil {
		t.Fatal("Packages map should be initialized")
	}
}

func TestNewStoreUsesInventoryPath(t *testing.T) {
	t.Setenv("PKGLINE_ROOT", t.TempDir())
	s := NewStore()
	if s.filePath == "" {
		t.Fatal("expected inventory path")
	}
}
