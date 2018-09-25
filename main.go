package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
)

const (
	targetBits   = 24
	maxNonce     = math.MaxInt64
	blockdb      = "blockdb"
	blocksBucket = "blocksBucket"
)

//Block is a type of data Block
type Block struct {
	Timestamp                 int64
	Data, PrevBlockHash, Hash []byte
	Nonce                     int
}

//BlockChain is a type of blockchains
type BlockChain struct {
	tip []byte
	db  *bolt.DB
}

//ProofOfWork is a type of block with target
type ProofOfWork struct {
	block  *Block
	target *big.Int
}

//BlockChainIterator is a type of Inumerator of block chain
type BlockChainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

type CLI struct {
	bc *BlockChain
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  addblock -data BLOCK_DATA - add a block to the blockchain")
	fmt.Println("  printchain - print all the blocks of the blockchain")
}

func (cli *CLI) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) Run() {

	cli.validateArgs()

	addBlockCmd := flag.NewFlagSet("addblock", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	addBlockData := addBlockCmd.String("data", "", "Block data")

	switch os.Args[1] {
	case "addblock":
		addBlockCmd.Parse(os.Args[2:])
	case "printchain":
		printChainCmd.Parse(os.Args[2:])
	default:
		cli.printUsage()
		os.Exit(1)
	}

	if addBlockCmd.Parsed() {
		if *addBlockData == "" {
			addBlockCmd.Usage()
			os.Exit(1)
		}
		cli.addBlock(*addBlockData)
	}

	if printChainCmd.Parsed() {
		cli.printChain()
	}

}

func (cli *CLI) addBlock(data string) {
	cli.bc.AddBlock(data)
	fmt.Println("Success!")
}

func (cli *CLI) printChain() {
	bci := cli.bc.Iterator()

	for {
		block := bci.Next()
		fmt.Println()
		fmt.Printf("Prev. hash :%x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("Pow: %s \n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
}

//Iterator is a Iterator Inumerator of block chain
func (bc *BlockChain) Iterator() *BlockChainIterator {
	bci := &BlockChainIterator{bc.tip, bc.db}
	return bci
}

//Next is a function of Iterator
func (i *BlockChainIterator) Next() *Block {
	var block *Block

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encodeBlock := b.Get(i.currentHash)
		block = Deserialize(encodeBlock)

		return nil
	})
	if err != nil {
		panic(err)
	}
	i.currentHash = block.PrevBlockHash
	return block
}

//SetHash is a function of make hash of block
func (b *Block) SetHash() {
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))
	headers := bytes.Join([][]byte{b.PrevBlockHash, b.Data, timestamp}, []byte{})
	hash := sha256.Sum256(headers)
	b.Hash = hash[:]
}

//NewBlock is a function of create a new block
func NewBlock(data string, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), []byte(data), prevBlockHash, []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()

	block.Hash = hash
	block.Nonce = nonce

	return block
}

//AddBlock is a function of add block to blockchain
func (bc *BlockChain) AddBlock(data string) {
	var lastHash []byte

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("l"))
		return nil
	})

	if err != nil {
		fmt.Println("AddBlock1" + err.Error())
	}

	newBlock := NewBlock(data, lastHash)

	err = bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(newBlock.Hash, newBlock.Serialize())
		err = b.Put([]byte("l"), newBlock.Hash)
		bc.tip = newBlock.Hash
		if err != nil {
			fmt.Println("AddBlock2" + err.Error())
		}
		return nil
	})

	if err != nil {
		fmt.Println("AddBlock3" + err.Error())
	}
}

//NewBlockChain is a function of create a block chain
func NewBlockChain() *BlockChain {
	var tip []byte
	db, err := bolt.Open(blockdb, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			fmt.Println("No existing blockchain found. Creating a new one...")
			genesis := NewGenesisBlock()
			b, err := tx.CreateBucket([]byte(blocksBucket))
			if err != nil {
				log.Panic(err)
			}
			err = b.Put(genesis.Hash, genesis.Serialize())
			if err != nil {
				log.Panic(err)
			}
			err = b.Put([]byte("l"), genesis.Hash)
			if err != nil {
				log.Panic(err)
			}
			tip = genesis.Hash
		} else {
			tip = b.Get([]byte("l"))
		}
		return nil
	})

	bc := BlockChain{tip, db}
	return &bc
	//return &BlockChain{[]*Block{NewGenesisBlock()}}
}

//NewGenesisBlock is a function of create a Genesis Block
func NewGenesisBlock() *Block {
	return NewBlock("This is Genesis Block!", []byte{})
}

//NewProofOfWork is a function if calculate of the block hash
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))
	log.Printf("target: %d", target)
	pow := &ProofOfWork{b, target}
	return pow
}

//IntToHex make int64 to (byte array)
func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

//prepareData is a function of return proofofwork data
func (pow *ProofOfWork) prepareData(nonce int) []byte {
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			pow.block.Data,
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(targetBits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)
	return data
}

//Run is a function of running result for block
func (pow *ProofOfWork) Run() (int, []byte) {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0
	fmt.Println()
	fmt.Printf("Mining the block containing \"%s\" \r\n", pow.block.Data)
	for nonce < maxNonce {
		data := pow.prepareData(nonce)
		hash = sha256.Sum256(data)
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.target) == -1 {

			fmt.Printf("\r%x \r\n", hash)
			fmt.Printf("nonce is %d \r\n", nonce)
			break
		} else {
			nonce++
		}
	}
	fmt.Print("\n\n")

	return nonce, hash[:]
}

// Validate is a function of validate block mining result
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	return hashInt.Cmp(pow.target) == -1
}

//Serialize is a function of serialize block data to byte array
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(b)
	if err != nil {
		fmt.Printf("Serialize block error: %s\n", err.Error())
	}
	return result.Bytes()
}

// Deserialize is a function of deserialize of bytes data convert to blockchain struct data
func Deserialize(data []byte) *Block {
	var result Block
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&result)
	if err != nil {
		fmt.Printf("Deserialize block error: %s\n", err.Error())
	}
	return &result
}

func main() {
	bc := NewBlockChain()
	defer bc.db.Close()

	cli := CLI{bc}
	cli.Run()
	//cli.addBlock("11111")
	//cli.addBlock("22222")
	//cli.printChain()
}
