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

const (
	servflowPrefix = "servflow"
	kvPrefix       = "kv"
	envStorageKey  = "SERVFLOW_STORAGE_PATH"
)

func GetClient() (*Client, error) {
	return sync.OnceValues(func() (*Client, error) {
		if client != nil {
			return client, nil
		}

		var (
			err error
			db  *badger.DB
		)
		path := os.Getenv(envStorageKey)
		opts := badger.DefaultOptions(path)
		opts.Logger = nil
		if path == "" {
			db, err = badger.Open(opts.WithInMemory(true))
		} else {
			db, err = badger.Open(opts)
		}
		if err != nil {
			return nil, err
		}

		client = &Client{db: db}
		return client, nil
	})()
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

// Set stores a key-value pair in the database
func Set(key string, value string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	c, err := GetClient()
	if err != nil {
		return err
	}

	k := []byte(fmt.Sprintf("%s:%s:%s", servflowPrefix, kvPrefix, key))

	return c.db.Update(func(txn *badger.Txn) error {
		return txn.Set(k, []byte(value))
	})
}

// Get retrieves a value by key from the database
// Returns value and true if key exists, empty string and false if key doesn't exist
func Get(key string) (string, bool, error) {
	if key == "" {
		return "", false, errors.New("key cannot be empty")
	}

	c, err := GetClient()
	if err != nil {
		return "", false, err
	}

	k := []byte(fmt.Sprintf("%s:%s:%s", servflowPrefix, kvPrefix, key))

	var value []byte
	var found bool
	err = c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(k)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				found = false
				return nil // Return no error, key just doesn't exist
			}
			return err
		}

		found = true
		return item.Value(func(val []byte) error {
			value = append([]byte(nil), val...)
			return nil
		})
	})

	if err != nil {
		return "", false, err
	}

	return string(value), found, nil
}
