package currency

import "fmt"

type CoinbaseTransaction struct {
	txId      TxId
	TxOut     TxOut
	Timestamp int64
}

func (txn *CoinbaseTransaction) GetTxId() TxId {
	return txn.txId
}

func (txn *CoinbaseTransaction) GetTxIns() []TxIn {
	return nil
}

func (txn *CoinbaseTransaction) GetTxOuts() []TxOut {
	return []TxOut{txn.TxOut}
}

func (txn *CoinbaseTransaction) GetTimestamp() int64 {
	return txn.Timestamp
}

func (txn *CoinbaseTransaction) Validate() error {
	txId := ComputeTxId(txn)
	if txn.txId != txId {
		return fmt.Errorf("txId mismatch: %s != %s", txn.txId, txId)
	}
	return nil
}

func (txn CoinbaseTransaction) String() string {
	return fmt.Sprintf("(t=%d) [%s]\n%s\n", txn.Timestamp, txn.txId, txn.TxOut)
}
