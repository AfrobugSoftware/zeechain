package blockchain

import "log"

type Block struct {
	TimeStamp    int64
	Hash         []byte
	Transactions []*Transaction
	PrevHash     []byte
	Nonce        int64
	Height       int64
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
