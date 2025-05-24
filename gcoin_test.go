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

func TestComputeUtxos(t *testing.T) {
	address, wallet := NewAddressWalletPair()
	ledger := NewLedger(NewCoinbaseTransaction(address, 1))
	utxos := wallet.ComputeUtxos(ledger)
	if len(utxos) != 1 {
		t.Error(utxos)
	}
}
