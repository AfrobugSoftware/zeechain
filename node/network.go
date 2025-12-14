package node

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"zeechain/blockchain"

	"github.com/vrecan/death"
)

const (
	protocol      = "tcp"
	version       = 1
	commandLength = 12
)

var (
	nodeAddress      string
	mineAddress      string
	KnownNodeAddress []string
	blocksInTransit  = [][]byte{}
	memoryPool       = make(map[string]blockchain.Transaction)
	bufferPool       = sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}
)

type Addr struct {
	AddressList []string
}

type Block struct {
	AddrFrom string
	Block    []byte
}

type GetBlocks struct {
	AddrFrom string
}

type GetData struct {
	AddrFrom string
	Type     string
	Id       []byte
}

type Inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

type Tx struct {
	AddrFrom    string
	Transaction []byte
}

type Version struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

// always copy the data from the return of this function
func GobEncode(data any) []byte {
	buff := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buff.Reset()
		bufferPool.Put(buff)
	}()
	enc := gob.NewEncoder(buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func CommandToByte(cmd string) []byte {
	var cb [commandLength]byte
	for i, c := range cmd {
		cb[i] = byte(c)
	}
	return cb[:]
}

func BytesToCommand(cb []byte) string {
	buf := new(bytes.Buffer)
	for _, c := range cb {
		if c != 0x00 {
			buf.WriteByte(c)
		}
	}
	return buf.String()
}

func HasNode(add string) bool {
	for _, node := range KnownNodeAddress {
		if node == add {
			return true
		}
	}
	return false
}

func ExtractCommand(request *bytes.Buffer) []byte {
	return request.Next(commandLength)
}

func SaveKnownNodes() error {
	f, err := os.Create("./nodes.nd")
	if err != nil {
		return err
	}
	data := strings.Join(KnownNodeAddress, "\n")
	_, err = f.Write([]byte(data))
	defer f.Close()
	if err != nil {
		return err
	}
	return nil
}

func LoadKnownNodes() error {
	f, err := os.Open("./nodes.nd")
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		KnownNodeAddress = append(KnownNodeAddress, line)
	}
	return nil
}

func SendData(addr string, data []byte) {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		log.Printf("%s is not avaliable", addr)
		for i, node := range KnownNodeAddress {
			if node == addr {
				if i+1 < len(KnownNodeAddress) {
					KnownNodeAddress = append(KnownNodeAddress[:i], KnownNodeAddress[i+1:]...)
				} else {
					KnownNodeAddress = KnownNodeAddress[:len(KnownNodeAddress)-1]
				}
				break
			}
		}
		return
	}
	defer conn.Close()
	if _, err := io.Copy(conn, bytes.NewReader(data)); err != nil {
		log.Panic(err)
	}
}

func SendAddr(address string) {
	nodes := Addr{KnownNodeAddress}
	nodes.AddressList = append(nodes.AddressList, nodeAddress)
	payload := GobEncode(nodes)
	request := append(CommandToByte("addr"), payload...)
	SendData(address, request)
}

func SendInv(address, kind string, item [][]byte) {
	payload := GobEncode(Inv{nodeAddress, kind, item})
	request := append(CommandToByte("inv"), payload...)
	SendData(address, request)
}

func RequestBlocks() {
	for _, node := range KnownNodeAddress {
		SendGetBlocks(node)
	}
}

func SendGetBlocks(addr string) {
	payload := GobEncode(GetBlocks{nodeAddress})
	request := append(CommandToByte("getblocks"), payload...)
	SendData(addr, request)
}
func SendGetData(address, kind string, id []byte) {
	payload := GobEncode(GetData{AddrFrom: nodeAddress, Type: kind, Id: id})
	request := append(CommandToByte("getdata"), payload...)
	SendData(address, request)
}

func SendBlock(addr string, block *blockchain.Block) {
	payload := GobEncode(Block{nodeAddress, block.Serialize()})
	request := append(CommandToByte("block"), payload...)
	SendData(addr, request)
}

func SendTx(address string, tx *blockchain.Transaction) {
	payload := GobEncode(Tx{AddrFrom: nodeAddress, Transaction: tx.Serialize()})
	request := append(CommandToByte("tx"), payload...)
	SendData(address, request)
}

func SendVersion(addr string, chain *blockchain.Blockchain) {
	bestHeight := chain.GetBestHeight()
	payload := GobEncode(Version{Version: version, BestHeight: bestHeight, AddrFrom: nodeAddress})
	request := append(CommandToByte("version"), payload...)
	SendData(addr, request)
}

func HandleAddr(req *bytes.Buffer) {
	var payload Addr
	dec := gob.NewDecoder(req)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	KnownNodeAddress = append(KnownNodeAddress, payload.AddressList...)
	fmt.Printf("there are %d known nodes\n", len(KnownNodeAddress))
	SaveKnownNodes()
	RequestBlocks()
}

func Handleblocks(req *bytes.Buffer, chain *blockchain.Blockchain) {
	var payload Block
	dec := gob.NewDecoder(req)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	block := blockchain.DeserializeBlock(bytes.NewReader(payload.Block))
	fmt.Println("Recevied a new block!")
	chain.AddBlock(block)
	fmt.Printf("Added block: %s\n", block.Hash)

	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)
		blocksInTransit = blocksInTransit[:1]
	} else {
		utxoSet := blockchain.UTXOSet{Chain: chain}
		utxoSet.ReIndex()
	}
}

func HandleGetData(req *bytes.Buffer, chain *blockchain.Blockchain) {
	var payload GetData
	dec := gob.NewDecoder(req)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	switch payload.Type {
	case "block":
		block, err := chain.GetBlock(payload.Id)
		if err != nil {
			log.Printf("%v\n", err)
			return
		}
		SendBlock(payload.AddrFrom, &block)
	case "tx":
		txId := hex.EncodeToString(payload.Id)
		tx := memoryPool[txId]
		SendTx(payload.AddrFrom, &tx)
	}
}

func HandleInv(req *bytes.Buffer, chain *blockchain.Blockchain) {
	var payload Inv
	dec := gob.NewDecoder(req)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	fmt.Printf("Recevied inventory with %d %s\n", len(payload.Items), payload.Type)
	switch payload.Type {
	case "block":
		blocksInTransit = payload.Items
		blockHash := payload.Items[0]
		SendGetData(payload.AddrFrom, "block", blockHash)
		for i, b := range blocksInTransit {
			if bytes.Equal(b, blockHash) {
				if i+1 < len(blocksInTransit) {
					blocksInTransit = append(blocksInTransit[:i], blocksInTransit[i+1:]...)
				} else {
					blocksInTransit = blocksInTransit[:len(blocksInTransit)-1]
				}
				break
			}
		}
	case "tx":
		txId := payload.Items[0]
		if memoryPool[hex.EncodeToString(txId)].ID == nil {
			SendGetData(payload.AddrFrom, "tx", txId)
		}
	}
}

func HandleGetBlocks(req *bytes.Buffer, chain *blockchain.Blockchain) {
	var payload GetBlocks
	dec := gob.NewDecoder(req)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	blocks := chain.GetBlockHashes()
	SendInv(payload.AddrFrom, "block", blocks)
}

func HandleTx(req *bytes.Buffer, chain *blockchain.Blockchain) {
	var payload Tx
	dec := gob.NewDecoder(req)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	tx := blockchain.Deserialize(bytes.NewReader(payload.Transaction))
	memoryPool[hex.EncodeToString(tx.ID)] = *tx
	fmt.Printf("%s, %d", nodeAddress, len(memoryPool))
	if nodeAddress == KnownNodeAddress[0] {
		for _, node := range KnownNodeAddress[1:] {
			if node != payload.AddrFrom {
				SendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		if len(memoryPool) >= 2 && len(mineAddress) > 0 {
			MineTx(chain)
		}
	}
}

func MineTx(chain *blockchain.Blockchain) {
	var txs []*blockchain.Transaction
	for _, tx := range memoryPool {
		if chain.VerifyTransactions(&tx) {
			txs = append(txs, &tx)
		}
	}
	if len(txs) == 0 {
		log.Println("All transactions are invalid")
	}
	cbTx := blockchain.CoinBaseTx(mineAddress, "")
	txs = append(txs, cbTx)
	newBlock, err := chain.MineBlock(txs)
	if err != nil {
		log.Panic(err)
	}
	UTXOSet := blockchain.UTXOSet{Chain: chain}
	UTXOSet.ReIndex()
	for _, tx := range memoryPool {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}
	for _, node := range KnownNodeAddress {
		if node != nodeAddress {
			SendInv(node, "block", [][]byte{newBlock.Hash})
		}
	}

	if len(memoryPool) > 0 {
		MineTx(chain)
	}
}

func HandleVersion(req *bytes.Buffer, chain *blockchain.Blockchain) {
	var payload Version
	dec := gob.NewDecoder(req)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	bestHeight := chain.GetBestHeight()
	otherHeight := payload.BestHeight
	if bestHeight < otherHeight {
		SendGetBlocks(payload.AddrFrom)
	} else if bestHeight > otherHeight {
		SendVersion(payload.AddrFrom, chain)
	}
	if !HasNode(payload.AddrFrom) {
		KnownNodeAddress = append(KnownNodeAddress, payload.AddrFrom)
	}
}

func HandleConnection(conn net.Conn, chain *blockchain.Blockchain) {
	buff := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		conn.Close()
		buff.Reset()
		bufferPool.Put(buff)
	}()
	_, err := buff.ReadFrom(conn)
	if err != nil && err != io.EOF {
		log.Panic(err)
	}
	command := BytesToCommand(ExtractCommand(buff))
	fmt.Printf("Recived %s command", command)
	switch command {
	case "addr":
		HandleAddr(buff)
	case "block":
		Handleblocks(buff, chain)
	case "inv":
		HandleInv(buff, chain)
	case "getblocks":
		HandleGetBlocks(buff, chain)
	case "tx":
		HandleTx(buff, chain)
	case "version":
		HandleVersion(buff, chain)
	default:
		return
	}
}

func StartServer(nodeAddr, nodeId, minerAddress string) {
	nodeAddress = nodeAddr
	mineAddress = minerAddress
	ln, err := net.Listen(protocol, nodeAddr)
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()
	chain := blockchain.ContinueBlockChain(nodeId)
	defer chain.Db.Close()
	go CloseDB(chain)
	LoadKnownNodes()
	if len(KnownNodeAddress) == 0 {
		KnownNodeAddress = append(KnownNodeAddress, nodeAddr)
	}
	if nodeAddr != KnownNodeAddress[0] {
		SendVersion(KnownNodeAddress[0], chain)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}
		go HandleConnection(conn, chain)
	}
}

func CloseDB(chain *blockchain.Blockchain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Db.Close()
	})
}
