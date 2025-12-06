package wallet

import "github.com/btcsuite/btcutil/base58"

func EncodeBase58(in []byte) []byte {
	encode := base58.Encode(in)
	return []byte(encode)
}

func DecodeBase58(in []byte) []byte {
	return base58.Decode(string(in[:]))
}
