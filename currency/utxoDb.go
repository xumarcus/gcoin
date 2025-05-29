package currency

import (
	"cmp"
	"fmt"
	"slices"
)

type UtxoDb struct {
	uTxIns       map[Address]map[TxIn]struct{}
	mapTxInTxOut map[TxIn]TxOut
}

// Assume validated
func (utxoDb *UtxoDb) UpdateTxData(txData *TxData) {
	for _, txIn := range txData.TxIns {
		txOut := utxoDb.mapTxInTxOut[txIn]
		delete(utxoDb.uTxIns[txOut.Address], txIn)
	}

	txId := txData.Hash()
	for i, txOut := range txData.TxOuts {
		txIn := TxIn{TxId: txId, OutIdx: uint64(i)}
		s, ok := utxoDb.uTxIns[txOut.Address]
		if !ok {
			s = make(map[TxIn]struct{})
			utxoDb.uTxIns[txOut.Address] = s
		}
		s[txIn] = struct{}{}
		utxoDb.mapTxInTxOut[txIn] = txOut
	}
}

// Assume UpdateTxData(txData) is called prior
func (utxoDb *UtxoDb) UndoUpdateTxData(txData *TxData) {
	for _, txIn := range txData.TxIns {
		txOut := utxoDb.mapTxInTxOut[txIn]
		utxoDb.uTxIns[txOut.Address][txIn] = struct{}{}
	}

	txId := txData.Hash()
	for i, txOut := range txData.TxOuts {
		txIn := TxIn{TxId: txId, OutIdx: uint64(i)}
		delete(utxoDb.uTxIns[txOut.Address], txIn)
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
	s := utxoDb.uTxIns[address]
	for _, txIn := range txn.TxData.TxIns {
		_, ok := s[txIn]
		if !ok {
			return fmt.Errorf("txIn %v invalid", txIn)
		}
		txOut, ok := utxoDb.mapTxInTxOut[txIn]
		if !ok {
			return fmt.Errorf("txIn %v no txOut", txIn)
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

func (utxoDb *UtxoDb) FilterRegularTransactions(mempool []RegularTransaction) []RegularTransaction {
	var txns []RegularTransaction
	for _, txn := range mempool {
		if err := utxoDb.ValidateRegularTransaction(&txn); err != nil {
			continue
		}
		utxoDb.UpdateTxData(&txn.TxData)
		txns = append(txns, txn)
	}

	for _, txn := range txns {
		utxoDb.UndoUpdateTxData(&txn.TxData)
	}
	return txns
}

func (utxoDb *UtxoDb) AvailableFunds(address Address) uint64 {
	if uTxIns, ok := utxoDb.uTxIns[address]; ok {
		var funds uint64
		for txIn := range uTxIns {
			if txOut, ok := utxoDb.mapTxInTxOut[txIn]; ok {
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
		uTxIns:       make(map[Address]map[TxIn]struct{}),
		mapTxInTxOut: make(map[TxIn]TxOut)}
}

func NewUtxoDbFromChain(chain Chain) UtxoDb {
	utxoDb := NewUtxoDb()
	for _, b := range chain {
		utxoDb.UpdateFromBlockTransactions(&b.Data)
	}
	return utxoDb
}
