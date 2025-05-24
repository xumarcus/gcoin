package main

import (
	"testing"
)

func TestValidate(t *testing.T) {
	chain := NewChain([]int{0, 1, 2})
	err := chain.Validate()
	if err != nil {
		t.Error(err)
	}
}

func TestCumulativeDifficulty(t *testing.T) {
	chain := NewChain([]int{0, 1, 2})
	cd := chain.CumulativeDifficulty()
	if cd != 3 {
		t.Error(cd)
	}
}

func TestGetAvailableFunds(t *testing.T) {
	wallet := NewWallet()
	address := wallet.GetAddress()
	ledger := NewLedgerWithTransaction(NewCoinbaseTransaction(address, 1))
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
	ledger := NewLedgerWithTransaction(NewCoinbaseTransaction(address1, 3))
	txn, err := wallet1.MakeTransaction(&ledger, address2, 2)
	if err != nil {
		panic(err)
	}

	// temp
	chain := ledger.chain.AppendBlock([]Transaction{*txn})
	ledger = Ledger{chain: chain}
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
