package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"io"
	"log"
	"time"
)

type Transaction struct {
	Date    time.Time
	ID      []byte
	Inputs  []TransInput
	Outputs []TransOutput
}

func (tx *Transaction) Hash() []byte {
	tempId := tx.ID
	tx.ID = []byte{}
	hash := sha256.Sum256(tx.Serialize())
	tx.ID = tempId
	return hash[:]
}

func (tx *Transaction) Serialize() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic("Cannot serialize transaction")
		return nil
	}
	return buf.Bytes()
}

func Deserialize(data io.Reader) *Transaction {
	var trans Transaction
	dec := gob.NewDecoder(data)
	err := dec.Decode(&trans)
	if err != nil {
		log.Panicln("could not deserialize transactions")
		return nil
	}
	return &trans
}

func CoinBaseTx(to, data string) *Transaction {
	if data == "" {
		randData := make([]byte, 24)
		_, err := rand.Read(randData)
		if err != nil {
			log.Panicln("failed to create random number for the base token")
			return nil
		}
	}
	in := TransInput{
		ID:        nil,
		OutId:     -1,
		Signature: nil,
		PubKey:    []byte(data),
	}
	out := NewTransOutput(10, to)
	trans := &Transaction{
		Date:    time.Now(),
		ID:      nil,
		Inputs:  []TransInput{in},
		Outputs: []TransOutput{*out},
	}
	trans.ID = trans.Hash()
	return trans
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].ID) == 0 && tx.Inputs[0].OutId == -1
}

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTxs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}
	for _, in := range tx.Inputs {
		if prevTxs[hex.EncodeToString(in.ID)].ID == nil {
			//what is the point of this check, why would th e
			log.Panic("Previous transactions are void")
		}
	}
}

func (tx *Transaction) TrimmedCopy() Transaction {
	txInputs := make([]TransInput, 0, len(tx.Inputs))
	txOutputs := make([]TransOutput, 0, len(tx.Outputs))

	for _, in := range tx.Inputs {
		txInputs = append(txInputs, TransInput{in.ID, in.OutId, nil, nil})
	}
	for _, out := range tx.Outputs {
		txOutputs = append(txOutputs, TransOutput{out.Value, out.PubKeyHash})
	}
	return Transaction{tx.Date, tx.ID, txInputs, txOutputs}
}
