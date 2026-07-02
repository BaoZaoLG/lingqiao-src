package main

import "github.com/lingqiao/server/internal/storage"

type JSONStorage = storage.JSONStore

func NewJSONStorage(dir string) *JSONStorage {
	return storage.NewJSONStore(dir)
}
