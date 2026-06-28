package main

import "github.com/lingqiao/server/internal/storage"

// JSONStorage preserves the existing storage API while delegating to the
// platform storage package.
type JSONStorage struct {
	store storage.Store
}

func NewJSONStorage(dir string) *JSONStorage {
	return &JSONStorage{store: storage.NewJSONStore(dir)}
}

func (s *JSONStorage) Save(name string, v interface{}) error {
	return s.store.Save(name, v)
}

func (s *JSONStorage) Load(name string, v interface{}) error {
	return s.store.Load(name, v)
}
