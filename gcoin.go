package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"time"
)

type Validatable interface {
	Validate() error
}

// https://lhartikk.github.io/

type Hash [32]byte

type Block[T any] struct {
	index        uint64
	timestamp    int64
	data         T
	previousHash Hash
	hash         Hash
	difficulty   uint8
	nonce        uint64
}

type Chain[T any] []Block[T]

func (b *Block[T]) ComputeHash() Hash {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, b.index)
	binary.Write(&buf, binary.BigEndian, b.timestamp)
	binary.Write(&buf, binary.BigEndian, b.data)
	binary.Write(&buf, binary.BigEndian, b.previousHash)
	binary.Write(&buf, binary.BigEndian, b.difficulty)
	binary.Write(&buf, binary.BigEndian, b.nonce)
	return sha256.Sum256(buf.Bytes())
}

func NewBlock[T any](data T) Block[T] {
	b := Block[T]{
		index:        0,
		timestamp:    time.Now().UnixMilli(),
		data:         data,
		previousHash: [32]byte{},
		hash:         [32]byte{},
		difficulty:   0,
		nonce:        0}
	b.hash = b.ComputeHash()
	return b
}

func (hash *Hash) LeadingZeros() int {
	ans := 0
	for _, x := range hash {
		ans += bits.LeadingZeros8(x)
		if x != 0 {
			break
		}
	}
	return ans
}

func (chain Chain[T]) NextBlock(data T) Block[T] {
	last := chain[len(chain)-1]
	b := Block[T]{
		index:        last.index + 1,
		timestamp:    time.Now().UnixMilli(),
		data:         data,
		previousHash: last.hash,
		hash:         [32]byte{},
		difficulty:   0,
		nonce:        0}
	b.difficulty = chain.BlockDifficulty(&b)
	for {
		b.hash = b.ComputeHash()
		if uint8(b.hash.LeadingZeros()) < b.difficulty {
			b.nonce++
		} else {
			return b
		}
	}
}

func (chain Chain[T]) AppendBlock(data T) Chain[T] {
	return append(chain, chain.NextBlock(data))
}

func NewChain[T any](datas []T) Chain[T] {
	n := len(datas)
	chain := make([]Block[T], n)
	chain[0] = NewBlock(datas[0])
	for i := 1; i < n; i++ {
		chain[i] = Chain[T](chain[:i]).NextBlock(datas[i])
	}
	return chain
}

func (chain Chain[T]) BlockDifficulty(b *Block[T]) uint8 {
	const NUM_MILLISECONDS_PER_BLOCK_GENERATED = 200
	const NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT = 4
	const TIME_EXPECTED = NUM_MILLISECONDS_PER_BLOCK_GENERATED * NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT

	last := chain[len(chain)-1]
	if b.index%NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT != 0 {
		return last.difficulty
	}

	// expect b.index != 0
	timeTaken := b.timestamp - chain[b.index-NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT].timestamp
	if timeTaken > TIME_EXPECTED*2 {
		if last.difficulty > 0 {
			return last.difficulty - 1
		} else {
			return 0
		}
	} else if timeTaken < TIME_EXPECTED/2 {
		return last.difficulty + 1
	} else {
		return last.difficulty
	}
}

func (chain Chain[T]) Validate() error {
	for i := range chain {
		b := &chain[i]
		if uint64(i) != b.index {
			return fmt.Errorf("index mismatch")
		}
		if i != 0 {
			if chain[i-1].hash != b.previousHash {
				return fmt.Errorf("previousHash does not match prev")
			}
			if chain[i-1].timestamp-60 >= b.timestamp {
				return fmt.Errorf("time travel to the past")
			}
			if b.timestamp-60 >= time.Now().UnixMilli() {
				return fmt.Errorf("time travel to the future")
			}
		}
		if b.ComputeHash() != b.hash {
			return fmt.Errorf("hash does not match block")
		}
	}
	return nil
}

func (chain Chain[T]) CumulativeDifficulty() int {
	ans := 0
	for i := range chain {
		b := &chain[i]
		ans += 1 << b.difficulty
	}
	return ans
}

func (chain Chain[T]) Less(other Chain[T]) bool {
	return chain.CumulativeDifficulty() < other.CumulativeDifficulty()
}

type Address [32]byte

type TxId [32]byte

type TxOut struct {
	address Address
	amount  uint64
}

type TxIn struct {
	txId        TxId
	txnTxOutIdx uint64
}

type Witness struct {
	sig []byte
	pub []byte
}

func (witness *Witness) GetAddress() Address {
	return sha256.Sum256(witness.pub)
}

type Transaction struct {
	txId    TxId
	txOuts  []TxOut
	txIns   []TxIn
	witness Witness
}

func (txn *Transaction) Validate() error {
	if txn.txId != txn.ComputeTxId() {
		return fmt.Errorf("id mismatch")
	}

	Curve := elliptic.P256()
	X, Y := elliptic.UnmarshalCompressed(Curve, txn.witness.pub)
	pub := ecdsa.PublicKey{
		Curve: Curve,
		X:     X,
		Y:     Y,
	}
	if !ecdsa.VerifyASN1(&pub, txn.txId[:], txn.witness.sig) {
		return fmt.Errorf("fail to verify txId with witness")
	}

	// defer everything else to ledger.Validate()
	return nil
}

func (txn *Transaction) ComputeTxId() TxId {
	var buf bytes.Buffer

	for i := range txn.txOuts {
		txOut := &txn.txOuts[i]
		binary.Write(&buf, binary.BigEndian, txOut.address)
		binary.Write(&buf, binary.BigEndian, txOut.amount)
	}

	for i := range txn.txIns {
		txIn := &txn.txIns[i]
		binary.Write(&buf, binary.BigEndian, txIn.txId)
		binary.Write(&buf, binary.BigEndian, txIn.txnTxOutIdx)
	}

	// Ignore txId
	return sha256.Sum256(buf.Bytes())
}

type UTXO struct {
	unusedTxOut TxOut
	refTxIn     TxIn
}

// Note: nodes can discard transactions in chain after verification (pruning)
type Ledger struct {
	chain   Chain[[]Transaction] // SSoT, keep track of timestamps
	utxoDb  map[Address][]UTXO   // full, complete UTXO set
	mempool []Transaction        // unconfirmed transactions
}

func (ledger *Ledger) Validate() error {
	return nil // TODO
}

func (ledger *Ledger) ComputeUtxoDb() map[Address][]UTXO {
	utxoDb := make(map[Address][]UTXO)
	seen := make(map[TxIn]bool)
	for i := len(ledger.chain) - 1; i != -1; i-- {
		b := &ledger.chain[i]
		for j := range b.data {
			txn := &b.data[j]
			for k := range txn.txIns {
				txIn := txn.txIns[k]
				seen[txIn] = true
			}
			for k := range txn.txOuts {
				txOut := txn.txOuts[k]
				refTxIn := TxIn{
					txId:        txn.txId,
					txnTxOutIdx: uint64(k)}
				if !seen[refTxIn] {
					utxo := UTXO{
						unusedTxOut: txOut,
						refTxIn:     refTxIn}
					utxoDb[txOut.address] = append(utxoDb[txOut.address], utxo)
				}
			}
		}
	}
	return utxoDb
}

func NewLedgerWithTransaction(txn Transaction) Ledger {
	data := []Transaction{txn}
	chain := NewChain([][]Transaction{data})
	ledger := Ledger{chain: chain}
	ledger.utxoDb = ledger.ComputeUtxoDb()
	return ledger
}

func (uncommitted *Transaction) TransactionFee(ledger *Ledger) (uint64, error) {
	var ans uint64
	address := uncommitted.witness.GetAddress()
	utxos := ledger.utxoDb[address]
	for _, txIn := range uncommitted.txIns {
		idx := slices.IndexFunc(utxos, func(utxo UTXO) bool {
			return utxo.refTxIn == txIn
		})
		if idx != -1 {
			ans += utxos[idx].unusedTxOut.amount
		} else {
			return math.MaxUint64, fmt.Errorf("txIn %#v invalid", txIn)
		}
	}
	for _, txOut := range uncommitted.txOuts {
		if ans >= txOut.amount {
			ans -= txOut.amount
		} else {
			return math.MaxUint64, fmt.Errorf("remaining fee=%d < amount=%d", ans, txOut.amount)
		}
	}
	return ans, nil
}

/*
 * Wallet represents a cryptocurrency wallet holding a single private key.
 * It enables cryptographic operations across different blockchain ledgers.
 */
type Wallet ecdsa.PrivateKey

func (wallet *Wallet) GetPub() []byte {
	pub := wallet.PublicKey
	return elliptic.MarshalCompressed(pub.Curve, pub.X, pub.Y)
}

func (wallet *Wallet) GetAddress() Address {
	return sha256.Sum256(wallet.GetPub())
}

func (wallet *Wallet) MakeWitness(txId TxId) Witness {
	sig, err := ecdsa.SignASN1(rand.Reader, (*ecdsa.PrivateKey)(wallet), txId[:])
	if err != nil {
		panic(err)
	}
	return Witness{
		sig: sig,
		pub: wallet.GetPub()}
}

func NewWallet() Wallet {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return Wallet(*priv)
}

func (wallet *Wallet) GetAvailableFunds(ledger *Ledger) uint64 {
	var ans uint64
	address := wallet.GetAddress()
	for _, utxo := range ledger.utxoDb[address] {
		ans += utxo.unusedTxOut.amount
	}
	return ans
}

func (wallet *Wallet) MakeCoinbaseTransaction(amount uint64) Transaction {
	address := wallet.GetAddress()
	txOut := TxOut{address: address, amount: amount}
	txn := Transaction{txOuts: []TxOut{txOut}, txIns: []TxIn{}}
	txn.txId = txn.ComputeTxId()
	txn.witness = wallet.MakeWitness(txn.txId)
	return txn
}

func (wallet *Wallet) MakeTransaction(ledger *Ledger, receiverAddress Address, amount uint64) (*Transaction, error) {
	if amount == 0 {
		return nil, fmt.Errorf("amount is zero")
	}

	senderAddress := wallet.GetAddress()
	receiverTxOut := TxOut{address: receiverAddress, amount: amount}
	ans := Transaction{
		txOuts: []TxOut{receiverTxOut},
		txIns:  []TxIn{}}

	for _, utxo := range ledger.utxoDb[senderAddress] {
		ans.txIns = append(ans.txIns, utxo.refTxIn)
		if amount >= utxo.unusedTxOut.amount {
			amount -= utxo.unusedTxOut.amount
		} else {
			change := utxo.unusedTxOut.amount - amount
			ans.txOuts = append(ans.txOuts, TxOut{address: senderAddress, amount: change})
			amount = 0
		}
	}

	if amount > 0 {
		return nil, fmt.Errorf("insufficient funds")
	}

	ans.txId = ans.ComputeTxId()
	ans.witness = wallet.MakeWitness(ans.txId)
	return &ans, nil
}
