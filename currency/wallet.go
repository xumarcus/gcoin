package currency

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

/*
 * Wallet represents a cryptocurrency wallet holding a single private key.
 * It enables cryptographic operations across different blockchain ledgers.
 */
type Wallet ecdsa.PrivateKey

func NewWallet() Wallet {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return Wallet(*priv)
}

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

func (wallet *Wallet) MakeRegularTransaction(utxoDb *UtxoDb, recvAddress Address, amount uint64) (*RegularTransaction, error) {
	if amount == 0 {
		return nil, fmt.Errorf("nothing to send")
	}

	sendAddress := wallet.GetAddress()
	txn := RegularTransaction{
		txOuts: []TxOut{{address: recvAddress, amount: amount}}}

	for txIn := range utxoDb.uTxIns[sendAddress].Iter() {
		txn.txIns = append(txn.txIns, txIn)
		txOut, ok := utxoDb.uTxOuts[txIn]
		if !ok {
			return nil, fmt.Errorf("txIn %v invalid", txIn)
		}

		if amount > txOut.amount {
			amount -= txOut.amount
		} else {
			change := txOut.amount - amount
			if change != 0 {
				txn.txOuts = append(txn.txOuts, TxOut{address: sendAddress, amount: change})
			}
			amount = 0
			break
		}
	}

	if amount != 0 {
		return nil, fmt.Errorf("%s not enough funds", sendAddress)
	}

	txn.txId = ComputeTxId(&txn)
	txn.witness = wallet.MakeWitness(txn.txId)
	return &txn, nil
}
