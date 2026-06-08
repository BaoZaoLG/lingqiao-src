package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func TestJSONStoreSaveLoadAndBackup(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	if err := store.Save("sample", sample{Name: "one", N: 1}); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	if err := store.Save("sample", sample{Name: "two", N: 2}); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	var got sample
	if err := store.Load("sample", &got); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got.Name != "two" || got.N != 2 {
		t.Fatalf("loaded %#v, want second value", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "sample.json.bak")); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
}

func TestJSONStoreLoadMissingFileReturnsNil(t *testing.T) {
	store := NewJSONStore(t.TempDir())

	var got sample
	if err := store.Load("missing", &got); err != nil {
		t.Fatalf("Load missing returned error: %v", err)
	}
}

func TestJSONStoreRejectsInvalidName(t *testing.T) {
	store := NewJSONStore(t.TempDir())

	err := store.Save("../escape", sample{Name: "bad"})
	if err == nil {
		t.Fatal("Save should reject path traversal name")
	}
	if !strings.Contains(err.Error(), "invalid storage name") {
		t.Fatalf("error = %v, want invalid storage name", err)
	}
}
