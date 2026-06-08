package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Store interface {
	Save(name string, value any) error
	Load(name string, value any) error
}

type JSONStore struct {
	dir string
	mu  sync.Mutex
}

func NewJSONStore(dir string) *JSONStore {
	_ = os.MkdirAll(dir, 0755)
	return &JSONStore{dir: dir}
}

func (s *JSONStore) Save(name string, value any) error {
	if err := validateName(name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, name+".json")
	tmpPath := path + ".tmp"
	bakPath := path + ".bak"

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(bakPath)
		if err := os.Rename(path, bakPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	}
	return os.Rename(tmpPath, path)
}

func (s *JSONStore) Load(name string, value any) error {
	if err := validateName(name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(s.dir, name+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, value)
}

func validateName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid storage name: %q", name)
	}
	return nil
}
