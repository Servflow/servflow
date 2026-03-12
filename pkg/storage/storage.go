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
	db   *badger.DB
	once sync.Once
	mu   sync.Mutex
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
	var openErr error

	c.once.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		db, err := openDB()
		if err != nil {
			openErr = err
			return
		}
		c.db = db
	})

	if openErr != nil {
		return openErr
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return errors.New("failed to initialize client")
	}

	return nil
}

func isDBClosedError(err error) bool {
	return errors.Is(err, badger.ErrDBClosed)
}

func (c *Client) reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.db = nil
	c.once = sync.Once{}

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
	c.once = sync.Once{}
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

		_, err = withRetryOnClose(func(c *Client) (struct{}, error) {
			err := c.db.Update(func(txn *badger.Txn) error {
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

const maxSIze = 50

func GetLogEntriesByPrefix(prefix string, deserializeFunc func([]byte) (any, error)) ([]any, error) {
	if prefix == "" {
		return nil, errors.New("prefix cannot be empty")
	}
	bPrefix := []byte(fmt.Sprintf("%s:%s:", servflowPrefix, prefix))

	return withRetryOnClose(func(c *Client) ([]any, error) {
		result := make([]any, 0)
		err := c.db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchSize = 10
			it := txn.NewIterator(opts)
			defer it.Close()
			for it.Seek(bPrefix); it.ValidForPrefix(bPrefix); it.Next() {
				if len(result) >= maxSIze {
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
		return result, err
	})
}

func withRetryOnClose[T any](operation func(*Client) (T, error)) (T, error) {
	var zero T

	c, err := GetClient()
	if err != nil {
		return zero, err
	}

	result, err := operation(c)
	if isDBClosedError(err) {
		if resetErr := c.reset(); resetErr != nil {
			return zero, resetErr
		}

		result, err = operation(c)
	}

	return result, err
}

func Set(key string, value string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	k := []byte(fmt.Sprintf("%s:%s:%s", servflowPrefix, kvPrefix, key))

	_, err := withRetryOnClose(func(c *Client) (struct{}, error) {
		err := c.db.Update(func(txn *badger.Txn) error {
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

	result, err := withRetryOnClose(func(c *Client) (GetResult, error) {
		var value []byte
		var found bool

		err := c.db.View(func(txn *badger.Txn) error {
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
