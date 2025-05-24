package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
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

type Wallet []ecdsa.PrivateKey

func GetWitness(priv *ecdsa.PrivateKey) []byte {
	pub := priv.PublicKey
	return elliptic.MarshalCompressed(pub.Curve, pub.X, pub.Y)
}

func GetAddress(priv *ecdsa.PrivateKey) Address {
	return sha256.Sum256(GetWitness(priv))
}

func NewAddressWalletPair() (Address, Wallet) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return GetAddress(priv), Wallet([]ecdsa.PrivateKey{*priv})
}

type TxId [32]byte

type TxOut struct {
	address Address
	amount  uint64
}

type TxOutCursor struct {
	txId        TxId
	txnTxOutIdx uint64
}

type TxIn struct {
	uTxOutCursor TxOutCursor
	witness      []byte
	signature    []byte
}

/*
	address1 := HASH(PUBKEY1)
	address2 := HASH(PUBKEY2)
	Txn1 TxOut [address1]
	Txn2 TxOut [address2]
	Txn3 witnesses=[PUBKEY1, PUBKEY2]
		TxIn [outTxId=Txn1 txnTxOutIdx=0 signature=SIGN(address1, PRIVKEY1)]
		TxIn [outTxId=Txn2 txnTxOutIdx=0 signature=SIGN(address2, PRIVKEY2)]
*/

type Transaction struct {
	txId   TxId
	txOuts []TxOut
	txIns  []TxIn
}

func (txn *Transaction) Validate() error {
	if txn.txId != txn.ComputeTxId() {
		return fmt.Errorf("id mismatch")
	}
	// defer everything else to ledger.Validate()
	return nil
}

func NewCoinbaseTransaction(address Address, amount uint64) Transaction {
	txOut := TxOut{address: address, amount: amount}
	txn := Transaction{txOuts: []TxOut{txOut}, txIns: []TxIn{}}
	txn.txId = txn.ComputeTxId()
	return txn
}

type Ledger Chain[[]Transaction]

func NewLedger(txn Transaction) Ledger {
	data := []Transaction{txn}
	return Ledger(NewChain([][]Transaction{data}))
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
		binary.Write(&buf, binary.BigEndian, txIn.uTxOutCursor.txId)
		binary.Write(&buf, binary.BigEndian, txIn.uTxOutCursor.txnTxOutIdx)
		// Ignore witness and signature data
	}

	// Ignore txId
	return sha256.Sum256(buf.Bytes())
}

type UTXO struct {
	uTxOut    TxOut
	cursor    TxOutCursor
	walletIdx int
}

func (wallet Wallet) ComputeUtxos(ledger Ledger) []UTXO {
	var ans []UTXO

	var seen map[TxOutCursor]bool

	for i := len(ledger) - 1; i != -1; i-- {
		b := &ledger[i]
		for j := range b.data {
			txn := &b.data[j]
			for k := range txn.txIns {
				txIn := &txn.txIns[k]
				seen[txIn.uTxOutCursor] = true
			}
			for k := range txn.txOuts {
				txOut := txn.txOuts[k]
				walletIdx := slices.IndexFunc(wallet, func(priv ecdsa.PrivateKey) bool {
					address := GetAddress(&priv)
					return bytes.Equal(address[:], txOut.address[:])
				})
				if walletIdx != -1 {
					cursor := TxOutCursor{
						txId:        txn.txId,
						txnTxOutIdx: uint64(k)}
					if !seen[cursor] {
						utxo := UTXO{
							uTxOut:    txOut,
							cursor:    cursor,
							walletIdx: walletIdx}
						ans = append(ans, utxo)
					}
				}
			}
		}
	}
	return ans
}

func (wallet Wallet) MakeTransaction(ledger Ledger, receiverAddress Address, amount uint64, senderChangeAddress Address) (*Transaction, error) {
	if amount == 0 {
		return nil, fmt.Errorf("amount is zero")
	}

	receiverTxOut := TxOut{address: receiverAddress, amount: amount}
	ans := Transaction{
		txOuts: []TxOut{receiverTxOut},
		txIns:  []TxIn{}}
	utxos := wallet.ComputeUtxos(ledger)
	for i := range utxos {
		utxo := &utxos[i]
		priv := wallet[utxo.walletIdx]
		witness := GetWitness(&priv)
		signature, err := ecdsa.SignASN1(rand.Reader, &priv, utxo.uTxOut.address[:])
		if err != nil {
			panic(err)
		}
		txIn := TxIn{
			uTxOutCursor: utxo.cursor,
			witness:      witness,
			signature:    signature}
		ans.txIns = append(ans.txIns, txIn)
		if amount >= utxo.uTxOut.amount {
			amount -= utxo.uTxOut.amount
		} else {
			change := utxo.uTxOut.amount - amount
			ans.txOuts = append(ans.txOuts, TxOut{address: senderChangeAddress, amount: change})
			amount = 0
		}
	}

	if amount > 0 {
		return nil, fmt.Errorf("insufficient funds")
	}

	ans.txId = ans.ComputeTxId()
	return &ans, nil
}
