package main

import (
	"encoding/json"
	"os"
	"sync"
)

type Store struct {
	mu   sync.Mutex
	Data map[string][]map[string]any
}

func NewStore(file string) *Store {
	s := &Store{Data: map[string][]map[string]any{}}

	if b, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(b, &s.Data)
	}
	return s
}

func (s *Store) Save(file string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.MarshalIndent(s.Data, "", "  ")
	_ = os.WriteFile(file, b, 0644)
}
