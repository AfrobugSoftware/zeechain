package blockchain

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dgraph-io/badger"
)

const (
	dbPath      = "./tmp/blocks_%s"
	genesisData = "First Transaction from Genesis"
)

type Blockchain struct {
	LastHash []byte
	Db       *badger.DB
}

func DBExists(path string) bool {
	_, err := os.Stat("/MANIFEST")
	return os.IsExist(err)
}

func ContinueBlockChain(nodeId string) *Blockchain {
	path := fmt.Sprintf(dbPath, nodeId)
	db, err := openDB(path)
	if err != nil {
		log.Fatal(err)
	}
	var lastHash []byte
	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return &Blockchain{lastHash, db}
}

func InitBlockChain(address, nodeId string) *Blockchain {
	path := fmt.Sprintf(dbPath, nodeId)
	if DBExists(path) {
		//why done we continue the block chain here
		runtime.Goexit()
	}
}

func retry(dir string) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK")
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK": %s`, err)
	}
	retryOpts := badger.DefaultOptions(dir)
	retryOpts.Truncate = true
	db, err := badger.Open(retryOpts)
	return db, err
}

func openDB(dir string) (*badger.DB, error) {
	if db, err := badger.Open(badger.DefaultOptions(dir)); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err := retry(dir); err == nil {
				log.Println("database unlocked, value log truncated")
				return db, nil
			}
			log.Println("could not unlock database:", err)
		}
		return nil, err
	} else {
		return db, nil
	}
}
