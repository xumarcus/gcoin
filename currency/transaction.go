package currency

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

type Transaction interface {
	TxId() TxId
	TxIns() []TxIn
	TxOuts() []TxOut
}

func ComputeTxId(txn Transaction) TxId {
	var buf bytes.Buffer
	for _, txOut := range txn.TxOuts() {
		binary.Write(&buf, binary.BigEndian, txOut.address)
		binary.Write(&buf, binary.BigEndian, txOut.amount)
	}
	for _, txIn := range txn.TxIns() {
		binary.Write(&buf, binary.BigEndian, txIn.txId)
		binary.Write(&buf, binary.BigEndian, txIn.outIdx)
	}
	return sha256.Sum256(buf.Bytes())
}

func IsEqual(a Transaction, b Transaction) bool {
	return a.TxId() == b.TxId()
}
