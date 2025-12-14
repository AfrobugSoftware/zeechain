package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"log"

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

func (w Wallet) MarshalJSON() ([]byte, error) {
	mapStringAny := map[string]any{
		"PrivateKey": map[string]any{
			"D": w.PrivateKey.D,
			"PublicKey": map[string]any{
				"X": w.PrivateKey.PublicKey.X,
				"Y": w.PrivateKey.PublicKey.Y,
			},
			"X": w.PrivateKey.X,
			"Y": w.PrivateKey.Y,
		},
		"PublicKey": w.PublicKey,
	}
	return json.Marshal(mapStringAny)
}
