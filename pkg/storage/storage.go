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
}

var client *Client
var clientMutex sync.Mutex

const (
	servflowPrefix = "servflow"
	kvPrefix       = "kv"
	envStorageKey  = "SERVFLOW_STORAGE_PATH"
)

var getClientOnce sync.Once

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
	getClientOnce.Do(func() {
		clientMutex.Lock()
		defer clientMutex.Unlock()

		if client != nil {
			return
		}

		db, err := openDB()
		if err != nil {
			client = &Client{db: nil}
			return
		}

		client = &Client{db: db}
	})

	clientMutex.Lock()
	defer clientMutex.Unlock()

	if client == nil || client.db == nil {
		return nil, errors.New("failed to initialize client")
	}

	return client, nil
}

func isDBClosedError(err error) bool {
	return errors.Is(err, badger.ErrDBClosed)
}

func resetClient() error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	if client != nil && client.db != nil {
		client.db.Close()
	}

	client = nil
	getClientOnce = sync.Once{}

	db, err := openDB()
	if err != nil {
		return err
	}

	client = &Client{db: db}
	return nil
}

type Serializable interface {
	Serialize() ([]byte, error)
}

func (c *Client) Close() error {
	return c.db.Close()
}

func WriteToLog(key string, value []Serializable) error {
	c, err := GetClient()
	if err != nil {
		return err
	}

	for _, v := range value {
		b, err := v.Serialize()
		if err != nil {
			return err
		}

		ts := time.Now().UnixNano()
		k := []byte(fmt.Sprintf("%s:%s:%d", servflowPrefix, strings.Trim(key, ":"), ts))

		err = c.db.Update(func(txn *badger.Txn) error {
			return txn.Set(k, b)
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

	c, err := GetClient()
	if err != nil {
		return nil, err
	}

	result := make([]any, 0)
	err = c.db.View(func(txn *badger.Txn) error {
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
}

func withRetryOnClose[T any](operation func(*Client) (T, error)) (T, error) {
	var zero T

	c, err := GetClient()
	if err != nil {
		return zero, err
	}

	result, err := operation(c)
	if isDBClosedError(err) {
		if resetErr := resetClient(); resetErr != nil {
			return zero, resetErr
		}

		c, err = GetClient()
		if err != nil {
			return zero, err
		}

		result, err = operation(c)
	}

	return result, err
}

// Set stores a key-value pair in the database
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

// Get retrieves a value by key from the database
// Returns value and true if key exists, empty string and false if key doesn't exist
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
