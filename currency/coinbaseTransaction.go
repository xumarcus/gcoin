package currency

import (
	"fmt"
	"time"
)

type CoinbaseTransaction struct {
	TxId   TxId
	TxData TxData
}

func (txn *CoinbaseTransaction) Validate() error {
	txId := txn.TxData.Hash()
	if txn.TxId != txId {
		return fmt.Errorf("txId mismatch: %s != %s", txn.TxId, txId)
	}
	return nil
}

func (txn *CoinbaseTransaction) Amount() uint64 {
	return txn.TxData.TxOuts[0].Amount
}

func NewCoinbaseTransaction(address Address, amount uint64) CoinbaseTransaction {
	txData := TxData{
		TxOuts:    []TxOut{{address, amount}},
		Timestamp: time.Now().UnixMilli(),
	}
	return CoinbaseTransaction{
		TxId:   txData.Hash(),
		TxData: txData,
	}
}
