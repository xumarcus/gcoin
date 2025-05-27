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
	for _, txIn := range txn.GetTxIns() {
		txOut := utxoDb.uTxOuts[txIn]
		if utxoDb.uTxIns[txOut.Address] != nil {
			utxoDb.uTxIns[txOut.Address].Remove(txIn)
		}
		delete(utxoDb.uTxOuts, txIn)
	}
	for i, txOut := range txn.GetTxOuts() {
		txIn := TxIn{TxId: txn.GetTxId(), OutIdx: uint64(i)}
		if utxoDb.uTxIns[txOut.Address] == nil {
			utxoDb.uTxIns[txOut.Address] = mapset.NewThreadUnsafeSet[TxIn]()
		}
		utxoDb.uTxIns[txOut.Address].Add(txIn)
		utxoDb.uTxOuts[txIn] = txOut
	}
}

func (utxoDb *UtxoDb) UpdateFromBlockTransactions(bt *BlockTransactions) {
	utxoDb.UpdateTransaction(&bt.CTxn)
	for _, txn := range bt.RTxns {
		utxoDb.UpdateTransaction(&txn)
	}
}

func (utxoDb *UtxoDb) ValidateRegularTransaction(txn *RegularTransaction) error {
	var transactionFee uint64
	address := txn.witness.GetAddress()
	for _, txIn := range txn.TxIns {
		txOut, ok := utxoDb.uTxOuts[txIn]
		if address != txOut.Address {
			return fmt.Errorf("address mismatch")
		}
		if !ok {
			return fmt.Errorf("txIn %v invalid", txIn)
		}
		transactionFee += txOut.Amount
	}
	for _, txOut := range txn.TxOuts {
		if transactionFee >= txOut.Amount {
			transactionFee -= txOut.Amount
		} else {
			return fmt.Errorf("balance(%d) < amount(%d) for %v", transactionFee, txOut.Amount, txOut)
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
				funds += txOut.Amount
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
		utxoDb.UpdateTransaction(&b.Data.CTxn)
		for _, txn := range b.Data.RTxns {
			utxoDb.UpdateTransaction(&txn)
		}
	}
	return utxoDb
}
