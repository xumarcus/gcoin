package currency

import (
	"crypto/ecdsa"
	"fmt"
	"strings"
)

type RegularTransaction struct {
	transactionFee uint64
	txId           TxId
	TxIns          []TxIn
	TxOuts         []TxOut
	Timestamp      int64
	witness        Witness
}

func (txn *RegularTransaction) GetTxId() TxId {
	return txn.txId
}

func (txn *RegularTransaction) GetTxIns() []TxIn {
	return txn.TxIns
}

func (txn *RegularTransaction) GetTxOuts() []TxOut {
	return txn.TxOuts
}

func (txn *RegularTransaction) GetTimestamp() int64 {
	return txn.Timestamp
}

func (txn *RegularTransaction) Validate() error {
	txId := ComputeTxId(txn)
	if txn.txId != txId {
		return fmt.Errorf("txId mismatch: %s != %s", txn.txId, txId)
	}

	pub := Unmarshal(txn.witness.pub)
	if !ecdsa.VerifyASN1(&pub, txn.txId[:], txn.witness.sig) {
		return fmt.Errorf("txId cannot verify")
	}

	return nil
}

func (txn RegularTransaction) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[%s]\n", txn.txId))

	builder.WriteString(fmt.Sprintf("sender: %s\n", txn.witness.GetAddress()))

	builder.WriteString("txIns:\n")
	for i, txIn := range txn.TxIns {
		builder.WriteString(fmt.Sprintf("%d: %s\n", i, txIn))
	}

	builder.WriteString("txOuts:\n")
	for i, txOut := range txn.TxOuts {
		builder.WriteString(fmt.Sprintf("%d: %s\n", i, txOut))
	}

	return builder.String()
}
