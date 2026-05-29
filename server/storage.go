package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// JSONStorage provides thread-safe JSON file persistence.
type JSONStorage struct {
	dir  string
	mu   sync.Mutex
}

// NewJSONStorage creates a storage instance with the given data directory.
func NewJSONStorage(dir string) *JSONStorage {
	os.MkdirAll(dir, 0755)
	return &JSONStorage{dir: dir}
}

// Save writes data to a JSON file atomically with backup.
func (s *JSONStorage) Save(name string, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, name+".json")
	tmpPath := path + ".tmp"
	bakPath := path + ".bak"

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	// Create backup of current file before replacing
	if _, err := os.Stat(path); err == nil {
		os.Remove(bakPath)
		os.Rename(path, bakPath)
	}

	return os.Rename(tmpPath, path)
}

// Load reads data from a JSON file.
func (s *JSONStorage) Load(name string, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // not an error if file doesn't exist yet
		}
		return err
	}

	return json.Unmarshal(data, v)
}
