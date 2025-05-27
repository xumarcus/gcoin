package blockchain

import "testing"

func TestComputeHash(t *testing.T) {
	b1 := NewBlock([]int{1, 2})
	b2 := NewBlock([]int{1, 2})

	if b1.Timestamp != b2.Timestamp {
		t.Errorf("%v != %v", b1, b2)
	}
	h1 := b1.ComputeHash()
	h2 := b2.ComputeHash()
	if h1 != h2 {
		t.Errorf("hash %v != %v", h1, h2)
	}

	b3 := NewBlock([]int{2, 1})
	if b1.Timestamp != b3.Timestamp {
		t.Errorf("%v != %v", b1, b2)
	}
	h3 := b3.ComputeHash()
	if h1 == h3 {
		t.Errorf("hash equals")
	}
}
