package main

import (
	"os"
	"testing"
)

func TestStorageSaveLoad(t *testing.T) {
	dir, _ := os.MkdirTemp("", "storagetest")
	defer os.RemoveAll(dir)

	s := NewJSONStorage(dir)

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := TestData{Name: "test", Value: 42}
	err := s.Save("test_data", &original)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	var loaded TestData
	err = s.Load("test_data", &loaded)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Name != "test" || loaded.Value != 42 {
		t.Fatalf("data mismatch: got %+v", loaded)
	}
}

func TestStorageLoadNonExistent(t *testing.T) {
	dir, _ := os.MkdirTemp("", "storagetest")
	defer os.RemoveAll(dir)

	s := NewJSONStorage(dir)

	var loaded map[string]string
	err := s.Load("nonexistent", &loaded)
	if err != nil {
		t.Fatalf("Load of nonexistent file should return nil, got: %v", err)
	}
}

func TestStorageBackup(t *testing.T) {
	dir, _ := os.MkdirTemp("", "storagetest")
	defer os.RemoveAll(dir)

	s := NewJSONStorage(dir)

	type V struct{ X int `json:"x"` }

	s.Save("backup_test", &V{X: 1})
	s.Save("backup_test", &V{X: 2})

	// Check .bak file exists
	_, err := os.Stat(dir + "/backup_test.json.bak")
	if err != nil {
		t.Fatal("backup file should exist after second save")
	}
}
