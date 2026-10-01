// Package kv is the key-value store the get_key and store_key actions read and
// write. The host owns the store and sets it once at startup; the actions only
// see this interface.
package kv

import (
	"errors"
	"sync"
)

// ErrNoStore is returned when an action reads or writes before the host has
// set a store.
var ErrNoStore = errors.New("no key-value store set")

// Store is a string key-value store.
type Store interface {
	Get(key string) (value string, found bool, err error)
	Set(key, value string) error
}

var (
	mu    sync.RWMutex
	store Store
)

// SetStore sets the store every later Get and Set uses.
func SetStore(s Store) {
	mu.Lock()
	defer mu.Unlock()
	store = s
}

// Get reads key from the store.
func Get(key string) (string, bool, error) {
	s, err := current()
	if err != nil {
		return "", false, err
	}
	return s.Get(key)
}

// Set writes value under key in the store.
func Set(key, value string) error {
	s, err := current()
	if err != nil {
		return err
	}
	return s.Set(key, value)
}

func current() (Store, error) {
	mu.RLock()
	defer mu.RUnlock()
	if store == nil {
		return nil, ErrNoStore
	}
	return store, nil
}
