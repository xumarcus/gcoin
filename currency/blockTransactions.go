package currency

import (
	"fmt"
	"strings"
)

type BlockTransactions struct {
	cTxn  CoinbaseTransaction
	rTxns []RegularTransaction
}

func transactionFees(txns []RegularTransaction) uint64 {
	var fees uint64
	for _, txn := range txns {
		fees += txn.transactionFee
	}
	return fees
}

func NewBlockTransactions(txns []RegularTransaction, address Address) BlockTransactions {
	fees := transactionFees(txns)
	txOut := TxOut{address: address, amount: DEFAULT_COINBASE_AMOUNT + fees}
	cTxn := CoinbaseTransaction{txOut: txOut}
	cTxn.txId = ComputeTxId(&cTxn)
	return BlockTransactions{cTxn: cTxn, rTxns: txns}
}

func (bt *BlockTransactions) Validate() error {
	if err := bt.cTxn.Validate(); err != nil {
		return fmt.Errorf("coinbase: %w", err)
	}
	for i, txn := range bt.rTxns {
		if err := txn.Validate(); err != nil {
			return fmt.Errorf("%d: %s", i, &txn)
		}
	}
	fees := transactionFees(bt.rTxns)
	if bt.cTxn.txOut.amount != DEFAULT_COINBASE_AMOUNT+fees {
		return fmt.Errorf("reward mismatch")
	}
	return nil
}

func (bt BlockTransactions) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("cTxn: %s\n", &bt.cTxn))
	for i, txn := range bt.rTxns {
		builder.WriteString(fmt.Sprintf("rTxns[%d]: %s\n", i, &txn))
	}
	return builder.String()
}
