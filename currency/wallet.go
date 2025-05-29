package currency

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"
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

func (wallet *Wallet) sourceTxIns(utxoDb *UtxoDb, txData *TxData, amount uint64) (uint64, error) {
	address := wallet.GetAddress()
	for txIn := range utxoDb.uTxIns[address] {
		txData.TxIns = append(txData.TxIns, txIn)
		txOut, ok := utxoDb.mapTxInTxOut[txIn]
		if !ok {
			return 0, fmt.Errorf("txIn %v invalid", txIn)
		}
		if amount > txOut.Amount {
			amount -= txOut.Amount
		} else {
			return txOut.Amount - amount, nil
		}
	}
	if amount != 0 {
		return 0, fmt.Errorf("%s not enough funds", address)
	}
	return 0, nil
}

func (wallet *Wallet) MakeRegularTransaction(utxoDb *UtxoDb, recvAddress Address, amount uint64, transactionFee uint64) (*RegularTransaction, error) {
	if amount == 0 {
		return nil, fmt.Errorf("nothing to send")
	}
	txData := TxData{
		TxOuts:    []TxOut{{Address: recvAddress, Amount: amount}},
		Timestamp: time.Now().UnixMilli()}
	if change, err := wallet.sourceTxIns(utxoDb, &txData, amount+transactionFee); err != nil {
		return nil, err
	} else {
		if change != 0 {
			txOut := TxOut{Address: wallet.GetAddress(), Amount: change}
			txData.TxOuts = append(txData.TxOuts, txOut)
		}
	}
	txId := txData.Hash()
	txn := RegularTransaction{
		TransactionFee: transactionFee,
		TxId:           txId,
		TxData:         txData,
		Witness:        wallet.MakeWitness(txId)}
	return &txn, nil
}
