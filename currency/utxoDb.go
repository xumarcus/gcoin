package currency

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
)

type UtxoDb struct {
	uTxIns  map[Address]mapset.Set[TxIn]
	uTxOuts map[TxIn]TxOut
}

// Assume checked with ValidateRegularTransaction
func (utxoDb *UtxoDb) UpdateTransaction(txn Transaction) {
	for _, txIn := range txn.TxIns() {
		txOut := utxoDb.uTxOuts[txIn]
		if utxoDb.uTxIns[txOut.address] != nil {
			utxoDb.uTxIns[txOut.address].Remove(txIn)
		}
	}
	for i, txOut := range txn.TxOuts() {
		txIn := TxIn{txId: txn.TxId(), outIdx: uint64(i)}
		if utxoDb.uTxIns[txOut.address] == nil {
			utxoDb.uTxIns[txOut.address] = mapset.NewThreadUnsafeSet[TxIn]()
		}
		utxoDb.uTxIns[txOut.address].Add(txIn)
		utxoDb.uTxOuts[txIn] = txOut
	}
}

func (utxoDb *UtxoDb) UpdateFromBlockTransactions(bt *BlockTransactions) {
	utxoDb.UpdateTransaction(&bt.cTxn)
	for _, txn := range bt.rTxns {
		utxoDb.UpdateTransaction(&txn)
	}
}

func (utxoDb *UtxoDb) ValidateRegularTransaction(txn *RegularTransaction) error {
	var transactionFee uint64
	address := txn.witness.GetAddress()
	for _, txIn := range txn.txIns {
		txOut, ok := utxoDb.uTxOuts[txIn]
		if address != txOut.address {
			return fmt.Errorf("address mismatch")
		}
		if !ok {
			return fmt.Errorf("txIn %v invalid", txIn)
		}
		transactionFee += txOut.amount
	}
	for _, txOut := range txn.txOuts {
		if transactionFee >= txOut.amount {
			transactionFee -= txOut.amount
		} else {
			return fmt.Errorf("balance(%d) < amount(%d) for %v", transactionFee, txOut.amount, txOut)
		}
	}
	if transactionFee != txn.transactionFee {
		return fmt.Errorf("fee mismatch")
	}
	return nil
}

func (utxoDb *UtxoDb) AvailableFunds(address Address) uint64 {
	if uTxIns, ok := utxoDb.uTxIns[address]; ok {
		var funds uint64
		for txIn := range uTxIns.Iter() {
			if txOut, ok := utxoDb.uTxOuts[txIn]; ok {
				funds += txOut.amount
			} else {
				panic(fmt.Errorf("txIn %v not found", txIn))
			}

		}
		return funds
	}
	return 0
}

func NewUtxoDb() UtxoDb {
	return UtxoDb{
		uTxIns:  make(map[Address]mapset.Set[TxIn]),
		uTxOuts: make(map[TxIn]TxOut)}
}

func NewUtxoDbFromChain(chain Chain) UtxoDb {
	utxoDb := NewUtxoDb()
	for _, b := range chain {
		utxoDb.UpdateTransaction(&b.Data.cTxn)
		for _, txn := range b.Data.rTxns {
			utxoDb.UpdateTransaction(&txn)
		}
	}
	return utxoDb
}
