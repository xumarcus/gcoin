package currency

import (
	"crypto/ecdsa"
	"fmt"
)

type RegularTransaction struct {
	TransactionFee uint64
	TxId           TxId
	TxData         TxData
	Witness        Witness
}

func (txn *RegularTransaction) Validate() error {
	txId := txn.TxData.Hash()
	if txn.TxId != txId {
		return fmt.Errorf("txId mismatch: %s != %s", txn.TxId, txId)
	}

	pub := Unmarshal(txn.Witness.pub)
	if !ecdsa.VerifyASN1(&pub, txn.TxId[:], txn.Witness.sig) {
		return fmt.Errorf("txId cannot verify")
	}

	return nil
}
