package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
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
	var lastHash []byte
	db, err := openDB(path)
	if err != nil {
		runtime.Goexit()
	}
	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinBaseTx(address, genesisData)
		genesis := Genesis(cbtx)
		fmt.Println("Created genesis block")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		if err != nil {
			log.Panic(err)
		}
		lastHash = genesis.Hash
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return &Blockchain{
		lastHash,
		db,
	}
}

func (chain *Blockchain) AddBlock(b *Block) {
	err := chain.Db.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(b.Hash); err != badger.ErrKeyNotFound {
			//exists
			return nil
		}
		blockData := b.Serialize()
		err := txn.Set(b.Hash, blockData)
		if err != nil {
			log.Panic(err)
		}
		item, err := txn.Get(chain.LastHash)
		if err != nil {
			log.Panic(err)
		}
		var lastBlock *Block
		err = item.Value(func(val []byte) error {
			lastBlock = DeserializeBlock(bytes.NewReader(val))
			return nil
		})
		if err != nil {
			log.Panic(err)
		}
		if b.Height > lastBlock.Height {
			err := txn.Set([]byte("lh"), b.Hash)
			if err != nil {
				log.Panic(err)
			}
			chain.LastHash = b.Hash
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

func (chain *Blockchain) GetBestHeight() int {
	db := chain.Db
	var Height int
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
		}
		err = item.Value(func(val []byte) error {
			lb := DeserializeBlock(bytes.NewReader(val))
			Height = lb.Height
			return nil
		})
		return err
	})
	if err != nil {
		log.Panic(err)
	}
	return Height
}
func (chain *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	var block Block
	err := chain.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(blockHash)
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			block = *DeserializeBlock(bytes.NewReader(val))
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return block, err
	}
	return block, nil
}

func (chain *Blockchain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	iter := chain.Iterator()
	for {
		block := iter.Next()

		blocks = append(blocks, block.Hash)
		if len(block.PrevHash) == 0 {
			break
		}
	}
	return blocks

}

func (chain *Blockchain) MineBlock(transactions []*Transaction) (*Block, error) {
	var lashHash []byte
	var lastHeight int
	for _, tx := range transactions {
		if !chain.VerifyTransactions(tx) {
			return nil, errors.New("invalid transaction")
		}
	}
	err := chain.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			lashHash = val
			return nil
		})
		if err != nil {
			return err
		}
		item, err = txn.Get(lashHash)
		if err != nil {
			return err
		}
		var lastBlock *Block
		err = item.Value(func(val []byte) error {
			lastBlock = DeserializeBlock(bytes.NewReader(val))
			return nil
		})
		if err != nil {
			return err
		}
		lastHeight = lastBlock.Height
		return nil
	})
	if err != nil {
		return nil, err
	}
	newBlock := CreateBlock(transactions, lashHash, lastHeight+1)
	err = chain.Db.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			return nil
		}
		err = txn.Set([]byte("lh"), newBlock.Hash)
		if err != nil {
			return nil
		}
		chain.LastHash = newBlock.Hash
		return nil
	})
	if err != nil {
		return nil, err
	}
	return newBlock, nil
}

func (chain *Blockchain) FindTransction(Id []byte) (Transaction, error) {
	iter := chain.Iterator()
	for {
		block := iter.Next()
		for _, tx := range block.Transactions {
			if bytes.Equal(tx.ID, Id) {
				return *tx, nil
			}
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
	return Transaction{}, errors.New("Transaction not found")
}

func (chain *Blockchain) SignTransactions(tx *Transaction, privKey *ecdsa.PrivateKey) error {
	prevTxs := make(map[string]Transaction)
	for _, in := range tx.Inputs {
		prevTx, err := chain.FindTransction(in.ID)
		if err != nil {
			return err
		}
		prevTxs[hex.EncodeToString(prevTx.ID)] = prevTx
	}
	return tx.Sign(*privKey, prevTxs)
}

func (chain *Blockchain) VerifyTransactions(tx *Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}
	prevTxs := make(map[string]Transaction)
	for _, in := range tx.Inputs {
		prevTx, err := chain.FindTransction(in.ID)
		if err != nil {
			log.Panic(err)
		}
		prevTxs[hex.EncodeToString(prevTx.ID)] = prevTx
	}
	v, err := tx.Verify(prevTxs)
	if err != nil {
		log.Panic(err)
	}
	return v

}

func (chain *Blockchain) FindUTXO() map[string]TransOutputs {
	UTXO := make(map[string]TransOutputs)
	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()
	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txId := hex.EncodeToString(tx.ID)
		Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txId] != nil {
					for _, spentOut := range spentTXOs[txId] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				outs := UTXO[txId]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txId] = outs
			}
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					inTxId := hex.EncodeToString(in.ID)
					spentTXOs[inTxId] = append(spentTXOs[inTxId], int(in.OutId))
				}
			}
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
	return UTXO
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
