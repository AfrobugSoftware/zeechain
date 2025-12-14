package wallet

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// should be kept in a hsm or a hardware module using alias and authentications
// as a test we are putting these things inside a store.
type Wallets struct {
	Wallets map[string]*Wallet
}

const walletFile = "wallet_%s.wal"

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
	err = json.Unmarshal(filecontexts, ws)
	if err != nil {
		return nil
	}
	return nil
}

func (ws *Wallets) SaveFile(nodeId string) error {
	walletFile := fmt.Sprintf(walletFile, nodeId)
	data, err := json.Marshal(ws)
	if err != nil {
		return err
	}
	err = os.WriteFile(walletFile, data, 0644)
	return err
}

func (ws *Wallets) AddWallet() string {
	w := NewWallet()
	address := w.Address()
	ws.Wallets[string(address)] = w
	return string(address)
}

func (ws *Wallets) GetAllAddresses(nodeId string) []string {
	var addreses []string
	if len(ws.Wallets) == 0 {
		err := ws.LoadFile(nodeId)
		if err != nil {
			log.Panic(err)
		}
	}
	for k := range ws.Wallets {
		addreses = append(addreses, k)
	}
	return addreses
}

func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}
