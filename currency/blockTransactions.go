package currency

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"gcoin/util"
)

type BlockTransactions struct {
	CTxn  CoinbaseTransaction
	RTxns []RegularTransaction
}

func transactionFees(txns []RegularTransaction) uint64 {
	var fees uint64
	for _, txn := range txns {
		fees += txn.TransactionFee
	}
	return fees
}

func NewBlockTransactions(txns []RegularTransaction, address Address) BlockTransactions {
	fees := transactionFees(txns)
	return BlockTransactions{CTxn: NewCoinbaseTransaction(address, DEFAULT_COINBASE_AMOUNT+fees), RTxns: txns}
}

func (bt BlockTransactions) Validate() error {
	if err := bt.CTxn.Validate(); err != nil {
		return fmt.Errorf("coinbase: %w", err)
	}
	for i, txn := range bt.RTxns {
		if err := txn.Validate(); err != nil {
			return fmt.Errorf("%d: %w", i, err)
		}
	}
	if bt.CTxn.Amount() != DEFAULT_COINBASE_AMOUNT+transactionFees(bt.RTxns) {
		return fmt.Errorf("reward mismatch")
	}
	return nil
}

func (bt BlockTransactions) Hash() util.Hash {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, bt.CTxn.TxId)
	for _, txn := range bt.RTxns {
		binary.Write(&buf, binary.BigEndian, txn.TxId)
	}
	return sha256.Sum256(buf.Bytes())
}
