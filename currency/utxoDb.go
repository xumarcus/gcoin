package currency

import (
	"cmp"
	"fmt"
	"slices"

	mapset "github.com/deckarep/golang-set/v2"
)

type UtxoDb struct {
	uTxIns  map[Address]mapset.Set[TxIn]
	uTxOuts map[TxIn]TxOut
}

// Assume validated
func (utxoDb *UtxoDb) UpdateTxData(txData *TxData) {
	for _, txIn := range txData.TxIns {
		txOut := utxoDb.uTxOuts[txIn]
		if utxoDb.uTxIns[txOut.Address] != nil {
			utxoDb.uTxIns[txOut.Address].Remove(txIn)
		}
		delete(utxoDb.uTxOuts, txIn)
	}

	txId := txData.Hash()
	for i, txOut := range txData.TxOuts {
		txIn := TxIn{TxId: txId, OutIdx: uint64(i)}
		if utxoDb.uTxIns[txOut.Address] == nil {
			// When utxoDb is deep copied, the mutex in NewSet will be copied too
			// Hence we must use NewThreadUnsafeSet here
			utxoDb.uTxIns[txOut.Address] = mapset.NewThreadUnsafeSet[TxIn]()
		}
		utxoDb.uTxIns[txOut.Address].Add(txIn)
		utxoDb.uTxOuts[txIn] = txOut
	}
}

func (utxoDb *UtxoDb) UpdateFromBlockTransactions(bt *BlockTransactions) {
	utxoDb.UpdateTxData(&bt.CTxn.TxData)
	for _, txn := range bt.RTxns {
		utxoDb.UpdateTxData(&txn.TxData)
	}
}

// Assume txn.Validate() == nil
func (utxoDb *UtxoDb) ValidateRegularTransaction(txn *RegularTransaction) error {
	var transactionFee uint64
	address := txn.Witness.GetAddress()
	for _, txIn := range txn.TxData.TxIns {
		txOut, ok := utxoDb.uTxOuts[txIn]
		if address != txOut.Address {
			return fmt.Errorf("address mismatch")
		}
		if !ok {
			return fmt.Errorf("txIn %v invalid", txIn)
		}
		transactionFee += txOut.Amount
	}
	for _, txOut := range txn.TxData.TxOuts {
		if transactionFee >= txOut.Amount {
			transactionFee -= txOut.Amount
		} else {
			return fmt.Errorf("not enough funds")
		}
	}
	if transactionFee != txn.TransactionFee {
		return fmt.Errorf("transactionFee mismatch")
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

type Tally TxOut

func (utxoDb *UtxoDb) Summary() []Tally {
	var tallies []Tally
	for address := range utxoDb.uTxIns {
		amount := utxoDb.AvailableFunds(address)
		tallies = append(tallies, Tally{Address: address, Amount: amount})
	}
	slices.SortFunc(tallies, func(a Tally, b Tally) int {
		return cmp.Compare(a.Amount, b.Amount)
	})
	return tallies
}

func NewUtxoDb() UtxoDb {
	return UtxoDb{
		uTxIns:  make(map[Address]mapset.Set[TxIn]),
		uTxOuts: make(map[TxIn]TxOut)}
}

func NewUtxoDbFromChain(chain Chain) UtxoDb {
	utxoDb := NewUtxoDb()
	for _, b := range chain {
		utxoDb.UpdateFromBlockTransactions(&b.Data)
	}
	return utxoDb
}
