package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"math/big"
	"time"
	"zeechain/wallet"
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

func NewTransaction(w *wallet.Wallet, to string, amount int, UTXO *UTXOSet) *Transaction {
	return nil
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

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTxs map[string]Transaction) error {
	if tx.IsCoinbase() {
		return nil
	}
	for _, in := range tx.Inputs {
		if prevTxs[hex.EncodeToString(in.ID)].ID == nil {
			//what is the point of this check, why would th e
			return errors.New("previous transactions are void")
		}
	}
	txCopy := tx.TrimmedCopy()
	for inIdx, in := range txCopy.Inputs {
		prevTx := prevTxs[hex.EncodeToString(in.ID)]
		txCopy.Inputs[inIdx].Signature = nil
		txCopy.Inputs[inIdx].PubKey = prevTx.Outputs[in.OutId].PubKeyHash

		dataSign := txCopy.Serialize()
		hash := sha256.Sum256(dataSign)
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, hash[:])
		if err != nil {
			return err
		}
		tx.Inputs[inIdx].Signature = append(r.Bytes(), s.Bytes()...)
		txCopy.Inputs[inIdx].PubKey = nil
	}
	return nil
}

func (tx *Transaction) Verify(prevTxs map[string]Transaction) (bool, error) {
	if tx.IsCoinbase() {
		return true, nil
	}

	for _, in := range tx.Inputs {
		if prevTxs[hex.EncodeToString(in.ID)].ID == nil {
			return false, errors.New("previous transactions are void")
		}
	}
	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()
	for inIdx, in := range tx.Inputs {
		prevTx := prevTxs[hex.EncodeToString(in.ID)]
		txCopy.Inputs[inIdx].Signature = nil
		txCopy.Inputs[inIdx].PubKey = prevTx.Outputs[in.OutId].PubKeyHash
		r := big.Int{}
		s := big.Int{}
		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])
		x := big.Int{}
		y := big.Int{}
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])
		pubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}
		dataSign := txCopy.Serialize()
		hash := sha256.Sum256(dataSign)
		if !ecdsa.Verify(&pubKey, hash[:], &r, &s) {
			return false, nil
		}
		txCopy.Inputs[inIdx].PubKey = nil
	}
	return true, nil
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
