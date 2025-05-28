package currency

import (
	"gcoin/blockchain"
	"testing"
)

func TestValidateRegularTransaction(t *testing.T) {
	wallet1 := NewWallet()
	wallet2 := NewWallet()

	bt1 := NewBlockTransactions([]RegularTransaction{}, wallet1.GetAddress())
	chain1 := blockchain.NewChain([]BlockTransactions{bt1})
	utxoDb1 := NewUtxoDbFromChain(chain1)

	rt, err := wallet1.MakeRegularTransaction(&utxoDb1, wallet2.GetAddress(), 5, 1)
	if err != nil {
		t.Error(err)
	}

	err = utxoDb1.ValidateRegularTransaction(rt)
	if err != nil {
		t.Error(err)
	}

	bt2 := NewBlockTransactions([]RegularTransaction{*rt}, wallet1.GetAddress())
	b := chain1.NextUnmintedBlock(bt2)
	b.Mine()
	chain2, err := chain1.Append(b)
	if err != nil {
		t.Error(err)
	}

	utxoDb2 := NewUtxoDbFromChain(chain2)
	a1 := utxoDb2.AvailableFunds(wallet1.GetAddress())
	a2 := utxoDb2.AvailableFunds(wallet2.GetAddress())

	if a1 != 94 && a2 != 5 {
		t.Errorf("a1=%d a2=%d", a1, a2)
	}

	utxoDb1.UpdateTxData(&rt.TxData)
	// TODO check utxoDb1 equal utxoDb2

	err = utxoDb1.ValidateRegularTransaction(rt)

	if err == nil {
		t.Error("duplicate found")
	}
}
