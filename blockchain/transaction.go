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
	"fmt"
	"io"
	"log"
	"math/big"
	"strings"
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
	var inputs []TransInput
	var outputs []TransOutput

	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)
	acc, valudOutputs := UTXO.FindSpendableOutput(pubKeyHash, amount)
	if acc < amount {
		log.Fatal("insufficient funds in wallet")
	}
	for tId, outs := range valudOutputs {
		txId, err := hex.DecodeString(tId)
		if err != nil {
			log.Panic(err)
		}
		for _, out := range outs {
			inputs = append(inputs, TransInput{ID: txId, OutId: int64(out), Signature: nil, PubKey: w.PublicKey})
		}
	}
	outputs = append(outputs, *NewTransOutput(uint64(amount), to))
	if acc > amount {
		outputs = append(outputs, *NewTransOutput(uint64(acc-amount), string(w.Address())))
	}
	tx := Transaction{time.Now(), nil, inputs, outputs}
	tx.ID = tx.Hash()
	UTXO.Chain.SignTransactions(&tx, &w.PrivateKey)
	return &tx
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
		r, s, x, y := big.Int{}, big.Int{}, big.Int{}, big.Int{}
		sigLen := len(in.Signature)
		keyLen := len(in.PubKey)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])
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

func (tx Transaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
	lines = append(lines, fmt.Sprint(tx.Date))
	for i, input := range tx.Inputs {
		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:     %x", input.ID))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.OutId))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
	}

	for i, output := range tx.Outputs {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
	}

	return strings.Join(lines, "\n")
}
