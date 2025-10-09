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
	Deserialize([]byte) error
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

func GetEntriesByPrefix[T Serializable](prefix string, factory func() T) ([]T, error) {
	if prefix == "" {
		return nil, errors.New("prefix cannot be empty")
	}
	bPrefix := []byte(fmt.Sprintf("%s:%s:", servflowPrefix, prefix))

	c, err := GetClient()
	if err != nil {
		return nil, err
	}

	result := make([]T, 0)
	err = c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(bPrefix); it.ValidForPrefix(bPrefix); it.Next() {
			if len(result) >= maxSIze {
				return nil
			}
			s := factory()
			if err := it.Item().Value(func(val []byte) error {
				return s.Deserialize(val)
			}); err != nil {
				return err
			}
			result = append(result, s)
		}
		return nil
	})
	return result, err
}
