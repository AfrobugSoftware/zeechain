package wallet

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"os"
)

// should be kept in a hsm or a hardware module using alias and authentications
// as a test we are putting these things inside a store.
type Wallets struct {
	Wallets map[string]*Wallet
}

const walletFile = "./zee/wallet_%s.wal"

func CreateWallets(nodeId string) (*Wallets, error) {
	w := &Wallets{
		Wallets: make(map[string]*Wallet),
	}
	err := w.LoadFile(nodeId)
	return w, err
}

func (ws *Wallets) LoadFile(nodeId string) error {
	filepath := fmt.Sprintf(walletFile, nodeId)
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return err
	}
	filecontexts, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(filecontexts))
	err = decoder.Decode(ws)
	if err != nil {
		return nil
	}
	return nil
}

func (ws *Wallet) SaveFile(nodeId string) error {
	var buf bytes.Buffer
	walletFile := fmt.Sprintf(walletFile, nodeId)

	gob.Register(elliptic.P256())
	encode := gob.NewEncoder(&buf)
	err := encode.Encode(ws)
	if err != nil {
		return err
	}
	err = os.WriteFile(walletFile, buf.Bytes(), 0644)
	return err
}

func (ws *Wallets) AddWallet() string {
	w := NewWallet()
	address := w.Address()
	ws.Wallets[string(address)] = w
	return string(address)
}

func (ws *Wallets) GetAddress() []string {
	var addreses []string
	for k, _ := range ws.Wallets {
		addreses = append(addreses, k)
	}
	return addreses
}

func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}
