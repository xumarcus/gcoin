package util

import (
	"encoding/hex"
	"math/bits"
)

type Hash [32]byte

func (hash Hash) String() string {
	return hex.EncodeToString(hash[:])
}

func (hash Hash) LeadingZeros() int {
	cnt := 0
	for _, x := range hash {
		cnt += bits.LeadingZeros8(x)
		if x != 0 {
			break
		}
	}
	return cnt
}
