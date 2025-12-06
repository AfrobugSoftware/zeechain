package blockchain

import (
	"bytes"
	"encoding/gob"
	"log"
	"zeechain/wallet"
)

type TransInput struct {
	ID        []byte
	OutId     int64
	Signature []byte
	PubKey    []byte
}

type TransOutput struct {
	Value      uint64
	PubKeyHash []byte
}

type TransOutputs struct {
	Outputs []TransOutput
}

func (tx *TransInput) UsesKey(pubKeyHash []byte) bool {
	inHash := wallet.PublicKeyHash(tx.PubKey)
	return bytes.Equal(inHash, pubKeyHash)
}

func (tx *TransOutput) Lock(address []byte) {
	pubKeyHash := wallet.DecodeBase58(address)
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-wallet.ChecksumLength]
	tx.PubKeyHash = pubKeyHash
}

func (tx *TransOutput) IsLockedWIthKey(pubKeyHash []byte) bool {
	return bytes.Equal(tx.PubKeyHash, pubKeyHash)
}

func NewTransOutput(value uint64, address string) *TransOutput {
	out := &TransOutput{value, nil}
	out.Lock([]byte(address))
	return out
}

func (txos TransOutputs) Serialize() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&txos)
	if err != nil {
		log.Panicln("error in serialize")
		return nil
	}
	return buf.Bytes()
}

func DeserialzeOutputs(data []byte) *TransOutputs {
	var txos TransOutputs
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&txos)
	if err != nil {
		log.Panic("Cannot deserialize the trans outputs")
		return nil
	}
	return &txos
}
