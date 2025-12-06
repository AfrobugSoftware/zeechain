package blockchain

var (
	utxoPrefix   = []byte("utfo-")
	prefixLength = len(utxoPrefix)
)

type UTXOSet struct {
	Chain *Blockchain
}
