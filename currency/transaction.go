package currency

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
)

type Transaction interface {
	GetTxId() TxId // For checking Equal
	GetTxIns() []TxIn
	GetTxOuts() []TxOut
	GetTimestamp() int64 // Transaction is Timestamped
}

func ComputeTxId(txn Transaction) TxId {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(txn); err != nil {
		panic(err)
	}
	return sha256.Sum256(buf.Bytes())
}
