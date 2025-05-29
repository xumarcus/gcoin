package currency

import (
	"gcoin/blockchain"
	"testing"
)

func TestValidateRegularTransaction(t *testing.T) {
	wallet1 := NewWallet()
	wallet2 := NewWallet()

	bt := NewBlockTransactions([]RegularTransaction{}, wallet1.GetAddress())
	chain := blockchain.NewChain([]BlockTransactions{bt})
	utxoDb := NewUtxoDbFromChain(chain)

	rt, err := wallet1.MakeRegularTransaction(&utxoDb, wallet2.GetAddress(), 5, 1)
	if err != nil {
		t.Error(err)
	}

	err = utxoDb.ValidateRegularTransaction(rt)
	if err != nil {
		t.Error(err)
	}

	utxoDb.UpdateTxData(&rt.TxData)
	err = utxoDb.ValidateRegularTransaction(rt)
	if err == nil {
		t.Error("duplicate found")
	}

	utxoDb.UndoUpdateTxData(&rt.TxData)
	err = utxoDb.ValidateRegularTransaction(rt)
	if err != nil {
		t.Error(err)
	}
}
