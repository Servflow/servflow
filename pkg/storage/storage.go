package storage

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type Client struct {
	db *badger.DB
	mu sync.Mutex
}

var client *Client
var clientMu sync.Mutex

const (
	servflowPrefix = "servflow"
	kvPrefix       = "kv"
	envStorageKey  = "SERVFLOW_STORAGE_PATH"
)

func openDB() (*badger.DB, error) {
	path := os.Getenv(envStorageKey)
	opts := badger.DefaultOptions(path)
	opts.Logger = nil
	if path == "" {
		return badger.Open(opts.WithInMemory(true))
	}
	return badger.Open(opts)
}

func GetClient() (*Client, error) {
	clientMu.Lock()
	defer clientMu.Unlock()

	if client == nil {
		client = &Client{}
	}

	if err := client.ensureOpen(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) ensureOpen() error {
	_, err := c.dbHandle()
	return err
}

// dbHandle returns the open badger DB, opening it if necessary. It takes the
// client mutex for the whole read-open-store sequence so that concurrent
// Close/reset (which mutate c.db under the same lock) can never race the field
// access. Callers must not hold c.mu.
func (c *Client) dbHandle() (*badger.DB, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		db, err := openDB()
		if err != nil {
			return nil, err
		}
		c.db = db
	}

	return c.db, nil
}

func isDBClosedError(err error) bool {
	return errors.Is(err, badger.ErrDBClosed)
}

func (c *Client) reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.db = nil

	db, err := openDB()
	if err != nil {
		return err
	}

	c.db = db
	return nil
}

type Serializable interface {
	Serialize() ([]byte, error)
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return nil
	}

	err := c.db.Close()
	c.db = nil
	return err
}

func WriteToLog(key string, value []Serializable) error {
	for _, v := range value {
		b, err := v.Serialize()
		if err != nil {
			return err
		}

		ts := time.Now().UnixNano()
		k := []byte(fmt.Sprintf("%s:%s:%d", servflowPrefix, strings.Trim(key, ":"), ts))

		_, err = withRetryOnClose(func(db *badger.DB) (struct{}, error) {
			err := db.Update(func(txn *badger.Txn) error {
				return txn.Set(k, b)
			})
			return struct{}{}, err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// GetLogEntriesByPrefix returns the most recent limit entries written under
// prefix, oldest first.
//
// Entries are keyed by write time, so "most recent" is the tail of the range:
// the scan runs backwards from the end of the prefix and stops once it has
// limit of them, then reverses what it collected so callers read a log in the
// order it happened. A thread far longer than limit therefore costs the same
// as a short one, and what a caller sees is its latest state rather than its
// first — which is what a conversation resuming after months of use needs.
//
// limit is the caller's window and must be positive: an unbounded read of a
// log that only grows is never what a caller wants.
func GetLogEntriesByPrefix(prefix string, limit int, deserializeFunc func([]byte) (any, error)) ([]any, error) {
	if prefix == "" {
		return nil, errors.New("prefix cannot be empty")
	}
	if limit <= 0 {
		return nil, errors.New("limit must be positive")
	}
	bPrefix := []byte(fmt.Sprintf("%s:%s:", servflowPrefix, prefix))

	return withRetryOnClose(func(db *badger.DB) ([]any, error) {
		result := make([]any, 0, limit)
		err := db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchSize = 10
			opts.Reverse = true
			it := txn.NewIterator(opts)
			defer it.Close()

			// A reverse seek lands on the largest key at or below its argument,
			// so it has to start above every key in the range: 0xFF sorts after
			// any byte a timestamp suffix can hold.
			for it.Seek(append(bPrefix, 0xFF)); it.ValidForPrefix(bPrefix); it.Next() {
				if len(result) >= limit {
					return nil
				}

				var item interface{}
				if err := it.Item().Value(func(val []byte) error {
					var err error
					item, err = deserializeFunc(val)
					return err
				}); err != nil {
					return err
				}
				result = append(result, item)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
		return result, nil
	})
}

// withRetryOnClose runs operation against the currently-open badger DB. The DB
// handle is snapshotted under the client mutex, so the operation never touches
// c.db directly and cannot race a concurrent Close/reset. If the DB was closed
// out from under us mid-operation, it is reopened once and the operation retried.
func withRetryOnClose[T any](operation func(*badger.DB) (T, error)) (T, error) {
	var zero T

	c, err := GetClient()
	if err != nil {
		return zero, err
	}

	db, err := c.dbHandle()
	if err != nil {
		return zero, err
	}

	result, err := operation(db)
	if isDBClosedError(err) {
		if resetErr := c.reset(); resetErr != nil {
			return zero, resetErr
		}

		if db, err = c.dbHandle(); err != nil {
			return zero, err
		}

		result, err = operation(db)
	}

	return result, err
}

func Set(key string, value string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	k := []byte(fmt.Sprintf("%s:%s:%s", servflowPrefix, kvPrefix, key))

	_, err := withRetryOnClose(func(db *badger.DB) (struct{}, error) {
		err := db.Update(func(txn *badger.Txn) error {
			return txn.Set(k, []byte(value))
		})
		return struct{}{}, err
	})

	return err
}

type GetResult struct {
	Value string
	Found bool
}

func Get(key string) (string, bool, error) {
	if key == "" {
		return "", false, errors.New("key cannot be empty")
	}

	k := []byte(fmt.Sprintf("%s:%s:%s", servflowPrefix, kvPrefix, key))

	result, err := withRetryOnClose(func(db *badger.DB) (GetResult, error) {
		var value []byte
		var found bool

		err := db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(k)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					found = false
					return nil
				}
				return err
			}

			found = true
			return item.Value(func(val []byte) error {
				value = append([]byte(nil), val...)
				return nil
			})
		})

		return GetResult{Value: string(value), Found: found}, err
	})

	return result.Value, result.Found, err
}
