package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"math/bits"
)

type Hash [32]byte

func (hash Hash) String() string {
	return hex.EncodeToString(hash[:])
}

func (hash Hash) MarshalText() ([]byte, error) {
	return []byte(hash.String()), nil
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

func NewHash[T any](data T) Hash {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		panic(err)
	}
	return sha256.Sum256(buf.Bytes())
}

type Hashable interface {
	Hash() Hash
}
