package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"pkgline/internal/path"
)

// PackageRecord represents an installed package stored in inventory.json
type PackageRecord struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	URI             string `json:"uri"`
	Language        string `json:"language"`
	Executable      string `json:"executable"`
	InstallType     string `json:"install_type"` // "native-go", "native-rust", "script"
	InstallScript   string `json:"install_script,omitempty"`
	UninstallScript string `json:"uninstall_script,omitempty"`
	AppDir          string `json:"app_dir"`
	BinPath         string `json:"bin_path"`
	GitCommit       string `json:"git_commit"`
	InstalledAt     string `json:"installed_at"`
	UpdatedAt       string `json:"updated_at"`
}

// Inventory holds map of all installed packages
type Inventory struct {
	Packages map[string]PackageRecord `json:"packages"`
}

// Store handles inventory storage operations
type Store struct {
	mu       sync.Mutex
	filePath string
}

// NewStore initializes a Store pointing to ~/.pkgline/inventory.json
func NewStore() *Store {
	return &Store{
		filePath: path.InventoryPath(),
	}
}

// NewStoreWithPath allows setting a custom inventory file path (for testing)
func NewStoreWithPath(p string) *Store {
	return &Store{
		filePath: p,
	}
}

func (s *Store) lockDir() string {
	return s.filePath + ".lock"
}

func (s *Store) withFileLock(fn func() error) error {
	lockDir := s.lockDir()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.Mkdir(lockDir, 0700)
		if err == nil {
			defer func() { _ = os.Remove(lockDir) }()
			return fn()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for inventory lock %s: %w", lockDir, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Load reads and parses the inventory file.
func (s *Store) Load() (*Inventory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv := &Inventory{
		Packages: make(map[string]PackageRecord),
	}

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return inv, nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read inventory file %s: %w", s.filePath, err)
	}

	if len(data) == 0 {
		return inv, nil
	}

	if err := json.Unmarshal(data, inv); err != nil {
		return nil, fmt.Errorf("failed to parse inventory JSON %s: %w", s.filePath, err)
	}

	if inv.Packages == nil {
		inv.Packages = make(map[string]PackageRecord)
	}

	return inv, nil
}

// Save atomically saves the inventory to disk.
func (s *Store) Save(inv *Inventory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if inv.Packages == nil {
		inv.Packages = make(map[string]PackageRecord)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for inventory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal inventory: %w", err)
	}

	tempFile := s.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary inventory file: %w", err)
	}

	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("failed to replace inventory file: %w", err)
	}

	return nil
}

// Get retrieves package by name.
func (s *Store) Get(name string) (PackageRecord, bool, error) {
	inv, err := s.Load()
	if err != nil {
		return PackageRecord{}, false, err
	}
	rec, exists := inv.Packages[name]
	return rec, exists, nil
}

// Set adds or updates a package in inventory.
func (s *Store) Set(rec PackageRecord) error {
	return s.withFileLock(func() error {
		inv, err := s.Load()
		if err != nil {
			return err
		}
		inv.Packages[rec.Name] = rec
		return s.Save(inv)
	})
}

// Remove deletes a package record from inventory.
func (s *Store) Remove(name string) error {
	return s.withFileLock(func() error {
		inv, err := s.Load()
		if err != nil {
			return err
		}
		delete(inv.Packages, name)
		return s.Save(inv)
	})
}

// List returns all package records sorted alphabetically by name.
func (s *Store) List() ([]PackageRecord, error) {
	inv, err := s.Load()
	if err != nil {
		return nil, err
	}

	records := make([]PackageRecord, 0, len(inv.Packages))
	for _, rec := range inv.Packages {
		records = append(records, rec)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})

	return records, nil
}
