package gcoin

import (
	"testing"
)

func TestValidate(t *testing.T) {
	chain := MakeChainFromData([]int{0, 1, 2})
	err := chain.Validate()
	if err != nil {
		t.Error(err)
	}
}

func TestGetCumulativeDifficulty(t *testing.T) {
	chain := MakeChainFromData([]int{0, 1, 2})
	cd := chain.GetCumulativeDifficulty()
	if cd != 3 {
		t.Error(cd)
	}
}

func TestGetAvailableFunds(t *testing.T) {
	wallet := NewWallet()
	ledger := MakeLedgerFromTransaction(wallet.MakeCoinbaseTransaction(1))
	amount := wallet.GetAvailableFunds(&ledger)
	if amount != 1 {
		t.Error(amount)
	}
}

func TestMakeTransaction(t *testing.T) {
	wallet1 := NewWallet()
	address1 := wallet1.GetAddress()
	wallet2 := NewWallet()
	address2 := wallet2.GetAddress()
	if address1 == address2 {
		t.Errorf("same address")
	}
	ledger := MakeLedgerFromTransaction(wallet1.MakeCoinbaseTransaction(3))
	txn, err := wallet1.MakeTransaction(&ledger, address2, 2)
	if err != nil {
		panic(err)
	}

	// TODO
	b := ledger.chain.NextBlock([]Transaction{*txn})
	b.Mine()
	ledger.chain = append(ledger.chain, b)
	ledger.utxoDb = ledger.ComputeUtxoDb()
	amount1 := wallet1.GetAvailableFunds(&ledger)
	if amount1 != 1 {
		t.Error(amount1)
	}
	amount2 := wallet2.GetAvailableFunds(&ledger)
	if amount2 != 2 {
		t.Error(amount2)
	}
}
