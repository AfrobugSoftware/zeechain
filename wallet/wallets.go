package wallet

import (
	"fmt"
	"log"
	"os"
)

// should be kept in a hsm or a hardware module using alias and authentications
// as a test we are putting these things inside a store.
type Wallets struct {
	Wallets map[string]*Wallet
}

var WalletDir string

func CreateWallets(nodeId string) (*Wallets, error) {
	w := &Wallets{
		Wallets: make(map[string]*Wallet),
	}
	err := w.LoadFile(nodeId)
	return w, err
}

func (ws *Wallets) LoadFile(nodeId string) error {
	dir := WalletDir
	if dir == "" {
		dir = fmt.Sprintf("%s%s", "wallet", nodeId)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			var w Wallet
			err := w.Load(fmt.Sprintf("%s/%s", dir, entry.Name()))
			if err != nil {
				log.Fatal(err)
			}
			ws.Wallets[string(w.Address())] = &w
		}
	}
	return nil
}

func (ws *Wallets) SaveFile(nodeId string) error {
	dir := WalletDir
	if dir == "" {
		dir = fmt.Sprintf("%s%s", "wallet", nodeId)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.Mkdir(dir, 0755)
		if err != nil {
			return err
		}
	}
	for _, w := range ws.Wallets {
		err := w.Save(dir)
		if err != nil {
			return err
		}
	}
	return nil
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
