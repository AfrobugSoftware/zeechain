package blockchain

import (
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"time"
)

type Block struct {
	TimeStamp    int64
	Hash         []byte
	Transactions []*Transaction
	PrevHash     []byte
	Nonce        int
	Height       int
}

func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Serialize())
	}
	tree, err := NewMerkleTree(txHashes)
	if err != nil {
		log.Panic(err)
	}
	return tree.RootNode.Data
}

func CreateBlock(txs []*Transaction, prevHash []byte, height int) *Block {
	block := &Block{time.Now().Unix(), []byte{}, txs, prevHash, 0, height}
	pow := NewProof(block)
	block.Nonce, block.Hash = pow.Run()
	return block
}

func Genesis(coinbase *Transaction) *Block {
	return CreateBlock([]*Transaction{coinbase}, []byte{}, 0)
}

func (b *Block) Serialize() []byte {
	var buf bytes.Buffer
	encode := gob.NewEncoder(&buf)
	err := encode.Encode(b)
	if err != nil {
		log.Panic("failed to serialize: %v", err)
	}
	return buf.Bytes()
}

func DeserializeBlock(in io.Reader) *Block {
	var block Block
	decode := gob.NewDecoder(in)
	err := decode.Decode(&block)
	if err != nil {
		log.Panic("failed to deserialzie: %v", err)
	}
	return &block
}
