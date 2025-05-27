package currency

import (
	"fmt"
	"strings"
	"time"
)

type BlockTransactions struct {
	CTxn  CoinbaseTransaction
	RTxns []RegularTransaction
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
	txOut := TxOut{Address: address, Amount: DEFAULT_COINBASE_AMOUNT + fees}
	cTxn := CoinbaseTransaction{TxOut: txOut, Timestamp: time.Now().UnixMilli()}
	cTxn.txId = ComputeTxId(&cTxn)
	return BlockTransactions{CTxn: cTxn, RTxns: txns}
}

func (bt *BlockTransactions) Validate() error {
	if err := bt.CTxn.Validate(); err != nil {
		return fmt.Errorf("coinbase: %w", err)
	}
	for i, txn := range bt.RTxns {
		if err := txn.Validate(); err != nil {
			return fmt.Errorf("%d: %s", i, &txn)
		}
	}
	fees := transactionFees(bt.RTxns)
	if bt.CTxn.TxOut.Amount != DEFAULT_COINBASE_AMOUNT+fees {
		return fmt.Errorf("reward mismatch")
	}
	return nil
}

func (bt BlockTransactions) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("cTxn: %s\n", &bt.CTxn))
	for i, txn := range bt.RTxns {
		builder.WriteString(fmt.Sprintf("rTxns[%d]: %s\n", i, &txn))
	}
	return builder.String()
}
