package currency

import "fmt"

type CoinbaseTransaction struct {
	txId  TxId
	txOut TxOut
}

func (txn *CoinbaseTransaction) TxId() TxId {
	return txn.txId
}

func (txn *CoinbaseTransaction) TxIns() []TxIn {
	return nil
}

func (txn *CoinbaseTransaction) TxOuts() []TxOut {
	return []TxOut{txn.txOut}
}

func (txn *CoinbaseTransaction) Validate() error {
	txId := ComputeTxId(txn)
	if txn.txId != txId {
		return fmt.Errorf("txId mismatch: %s != %s", txn.txId, txId)
	}
	return nil
}

func (txn CoinbaseTransaction) String() string {
	return fmt.Sprintf("[%s] %s", txn.txId, txn.txOut)
}
