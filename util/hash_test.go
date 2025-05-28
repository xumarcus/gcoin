package util

import "testing"

func TestNewHash(t *testing.T) {
	h1 := NewHash(1)
	h2 := NewHash(2)
	h3 := NewHash(h1)
	h4 := NewHash(h2)
	if h3 == h4 {
		t.Errorf("same hash")
	}
}
