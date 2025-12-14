package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/ripemd160"
)

const (
	ChecksumLength = 4
	Version        = byte(0x01)
)

type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

func Checksum(payload []byte) []byte {
	firstHash := sha256.Sum256(payload)
	secondHash := sha256.Sum256(firstHash[:])

	return secondHash[:ChecksumLength]
}

func PublicKeyHash(pubKey []byte) []byte {
	ph := sha256.Sum256(pubKey)
	hasher := ripemd160.New()
	_, err := hasher.Write(ph[:])
	if err != nil {
		log.Panic("Cannot produce pub hash")
	}
	rph := hasher.Sum(nil)
	return rph
}

func (w *Wallet) Address() []byte {
	pubKeyHash := PublicKeyHash(w.PublicKey)
	verisionHash := append([]byte{Version}, pubKeyHash...)
	checksum := Checksum(verisionHash)
	fullhash := append(verisionHash, checksum...)
	address := EncodeBase58(fullhash)
	return address
}

func ValidateAddress(address []byte) bool {
	pubKeyHash := DecodeBase58(address)
	actualChecksum := pubKeyHash[len(pubKeyHash)-ChecksumLength:]
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-ChecksumLength]
	targetChecksum := Checksum(append([]byte{version}, pubKeyHash...))

	return bytes.Equal(actualChecksum, targetChecksum)
}

func NewKeyPair() (*ecdsa.PrivateKey, []byte) {
	curve := elliptic.P256()
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic("could not generate keys for wallet")
	}
	pub := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)
	return private, pub
}

func NewWallet() *Wallet {
	private, pub := NewKeyPair()
	return &Wallet{
		PrivateKey: *private,
		PublicKey:  pub,
	}
}

func (w Wallet) Save(dir string) error {
	bs, err := x509.MarshalECPrivateKey(&w.PrivateKey)
	if err != nil {
		return err
	}
	pemBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: bs,
	}
	pemBytes := pem.EncodeToMemory(pemBlock)

	wfile := fmt.Sprintf("%s/%s.wal", dir, w.Address())
	err = os.WriteFile(wfile, pemBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (w *Wallet) Load(filename string) error {
	pemBytes, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		log.Fatal("Failed to decode PEM block containing private key")
	}
	pk, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	w.PublicKey = append(pk.PublicKey.X.Bytes(), pk.PublicKey.Y.Bytes()...)
	w.PrivateKey = *pk
	return nil
}
