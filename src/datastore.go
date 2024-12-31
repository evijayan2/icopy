package icopy

import (
	badger "github.com/dgraph-io/badger/v4"
)

func OpenBadgerDB(path string) (*badger.DB, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CloseBadgerDB(db *badger.DB) {
	db.Close()
}

func PutBadgerDB(db *badger.DB, hashKey string, fileNameValue string) error {
	key := []byte(hashKey)
	value := []byte(fileNameValue)
	return db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func GetBadgerDBValue(db *badger.DB, hashKey string) (string, error) {
	var value []byte
	key := []byte(hashKey)
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	return string(value), err
}

func IterateWithPrefix(db *badger.DB, prefix string) ([]string, error) {
	var keys []string
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		prefixKey := []byte(prefix)
		for it.Seek(prefixKey); it.ValidForPrefix(prefixKey); it.Next() {
			item := it.Item()
			k := item.Key()
			key := string(k)
			key = key[len(prefix)+1:]
			keys = append(keys, key)
		}
		return nil
	})
	return keys, err
}
