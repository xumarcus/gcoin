package main

import "testing"

func TestIsValid(t *testing.T) {
	b0 := NewBlock(0)
	b1 := NextBlock(&b0, 1)
	b2 := NextBlock(&b1, 2)
	chain := []Block[int]{b0, b1, b2}
	if !isValid(chain) {
		t.Fail()
	}
}
