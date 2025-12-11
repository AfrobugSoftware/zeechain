package blockchain

import (
	"bytes"
	"log"

	"github.com/dgraph-io/badger"
)

type BlockChainIterator struct {
	CurrentHash []byte
	Db          *badger.DB
}

func (chain *Blockchain) Iterator() *BlockChainIterator {
	return &BlockChainIterator{chain.LastHash, chain.Db}
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block
	err := iter.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		if err != nil {
			log.Panic(err)
		}
		var v []byte
		err = item.Value(func(val []byte) error {
			v = val
			return nil
		})
		if err != nil {
			log.Panic(err)
		}
		block = DeserializeBlock(bytes.NewReader(v))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	iter.CurrentHash = block.PrevHash
	return block
}
