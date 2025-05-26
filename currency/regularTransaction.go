package currency

import (
	"crypto/ecdsa"
	"fmt"
	"strings"
)

type RegularTransaction struct {
	transactionFee uint64
	txId           TxId
	txIns          []TxIn
	txOuts         []TxOut
	witness        Witness
}

func (txn *RegularTransaction) TxId() TxId {
	return txn.txId
}

func (txn *RegularTransaction) TxIns() []TxIn {
	return txn.txIns
}

func (txn *RegularTransaction) TxOuts() []TxOut {
	return txn.txOuts
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
	builder.WriteString(fmt.Sprintf("sender: %s\n", txn.witness.GetAddress()))

	builder.WriteString("txIns:\n")
	for i, txIn := range txn.txIns {
		builder.WriteString(fmt.Sprintf("%d: %s\n", i, txIn))
	}

	builder.WriteString("txOuts:\n")
	for i, txOut := range txn.txOuts {
		builder.WriteString(fmt.Sprintf("%d: %s\n", i, txOut))
	}

	return builder.String()
}
