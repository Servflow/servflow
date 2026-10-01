// Package kvtest provides an in-memory kv.Store for tests.
package kvtest

import "sync"

// Store is an in-memory kv.Store.
type Store struct {
	mu     sync.Mutex
	values map[string]string
}

// New returns an empty Store.
func New() *Store {
	return &Store{values: make(map[string]string)}
}

func (s *Store) Get(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[key]
	return v, ok, nil
}

func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}
