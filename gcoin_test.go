package main

import "testing"

func TestValidate(t *testing.T) {
	chain := NewChain([]int{0, 1, 2})
	err := validate(chain)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestCumulativeDifficulty(t *testing.T) {
	chain := NewChain([]int{0, 1, 2})
	cd := cumulativeDifficulty(chain)
	if cd != 3 {
		t.Fail()
	}
}
