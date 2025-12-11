package blockchain

import (
	"bytes"
	"encoding/hex"
	"log"

	"github.com/dgraph-io/badger"
)

var (
	utxoPrefix   = []byte("utfo-")
	prefixLength = len(utxoPrefix)
)

type UTXOSet struct {
	Chain *Blockchain
}

func (u UTXOSet) FindSpendableOutput(pubKeyHash []byte, amount int) (int, map[string][]int) {
	unspentOut := make(map[string][]int)
	accumulated := 0
	db := u.Chain.Db

	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			k := item.Key()
			var v []byte
			err := item.Value(func(val []byte) error {
				v = val
				return nil
			})
			if err != nil {
				return nil
			}
			k = bytes.TrimPrefix(k, utxoPrefix)
			txId := hex.EncodeToString(k)
			outs := DeserialzeOutputs(v)
			for outIdx, out := range outs.Outputs {
				if out.IsLockedWIthKey(pubKeyHash) && accumulated < amount {
					accumulated += int(out.Value)
					unspentOut[txId] = append(unspentOut[txId], outIdx)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return accumulated, unspentOut
}

func (u UTXOSet) ReIndex() {
	db := u.Chain.Db
	u.DeleteByPrefix(utxoPrefix)
	utxo := u.Chain.FindUTXO()
	err := db.Update(func(txn *badger.Txn) error {
		for txId, outs := range utxo {
			key, err := hex.DecodeString(txId)
			if err != nil {
				return err
			}
			key = append(utxoPrefix, key...)
			err = txn.Set(key, outs.Serialize())
			return err
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

func (u UTXOSet) FindUnspentTransactions(pubKeyHash []byte) []TransOutput {
	var UTXOs []TransOutput
	db := u.Chain.Db
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			var v []byte
			err := item.Value(func(val []byte) error {
				v = val
				return nil
			})
			if err != nil {
				return err
			}
			outs := DeserialzeOutputs(v)
			for _, out := range outs.Outputs {
				if out.IsLockedWIthKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return UTXOs
}

func (u UTXOSet) CountTransactions() int {
	db := u.Chain.Db
	counter := 0
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			counter++
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return counter
}

func (u *UTXOSet) Update(block *Block) {
	db := u.Chain.Db
	err := db.Update(func(txn *badger.Txn) error {
		for _, tx := range block.Transactions {
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					inId := append(utxoPrefix, in.ID...)
					item, err := txn.Get(inId)
					if err != nil {
						log.Panic(err)
					}
					var v []byte
					err = item.Value(func(val []byte) error {
						v = val
						return nil
					})
					outs := DeserialzeOutputs(v)
					updateOuts := TransOutputs{}
					for outIdx, out := range outs.Outputs {
						if outIdx != int(in.OutId) {
							updateOuts.Outputs = append(updateOuts.Outputs, out)
						}
					}
					if len(updateOuts.Outputs) == 0 {
						if err := txn.Delete(inId); err != nil {
							log.Panic(err)
						}
					} else {
						if err := txn.Set(inId, updateOuts.Serialize()); err != nil {
							log.Panic(err)
						}
					}
				}
				newOutputs := TransOutputs{}
				for _, out := range tx.Outputs {
					newOutputs.Outputs = append(newOutputs.Outputs, out)
				}
				txId := append(utxoPrefix, tx.ID...)
				if err := txn.Set(txId, newOutputs.Serialize()); err != nil {
					log.Panic(err)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

func (u *UTXOSet) DeleteByPrefix(prefix []byte) {
	db := u.Chain.Db
	deleteKeys := func(keysForDelete [][]byte) error {
		err := db.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	}
	collectSize := 100000
	db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++
			if keysCollected == collectSize {
				if err := deleteKeys(keysForDelete); err != nil {
					log.Panic(err)
				}
				keysForDelete = make([][]byte, 0, collectSize)
				keysCollected = 0
			}
		}
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
}
