package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"gcoin/util"
	"time"
)

type Block[T any] struct {
	CmDf      uint64
	Data      T
	Exp       uint8
	Index     uint64
	Nonce     uint64
	Timestamp int64
	prevHash  util.Hash
	hash      util.Hash
}

func NewBlock[T any](data T) Block[T] {
	b := Block[T]{
		Data:      data,
		Index:     0,
		Timestamp: time.Now().UnixMilli(),
		prevHash:  util.Hash{},
		hash:      util.Hash{},
		Exp:       0,
		CmDf:      1,
		Nonce:     0}
	b.hash = b.ComputeHash()
	return b
}

func (b Block[T]) Equal(other Block[T]) bool {
	return b.hash == other.hash
}

func (b *Block[T]) ComputeHash() util.Hash {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(b); err != nil {
		panic(err)
	}
	return sha256.Sum256(buf.Bytes())
}

func (b *Block[T]) GetHash() util.Hash {
	return b.hash
}

func (b *Block[T]) Mine() {
	for {
		b.hash = b.ComputeHash()
		if uint8(b.hash.LeadingZeros()) < b.Exp {
			b.Nonce++
		} else {
			return
		}
	}
}

func (b *Block[T]) Validate() error {
	if b.ComputeHash() != b.hash {
		return fmt.Errorf("hash mismatch")
	}
	if b.Timestamp-60 >= time.Now().UnixMilli() {
		return fmt.Errorf("is from future")
	}
	return nil
}

func (a *Block[T]) ValidateNextBlock(b *Block[T]) error {
	if b.Index != a.Index+1 {
		return fmt.Errorf("index mismatch")
	}
	if b.prevHash != a.hash {
		return fmt.Errorf("prevHash mismatch")
	}
	if b.CmDf != a.CmDf+(1<<b.Exp) {
		return fmt.Errorf("cd mismatch")
	}
	if b.Timestamp <= a.Timestamp-60 {
		return fmt.Errorf("is from past")
	}
	return nil
}

func (b Block[T]) String() string {
	return fmt.Sprintf("%d:(exp=%d)\nprevHash=%s\nHash=%s\n%v", b.Index, b.Exp, b.prevHash, b.hash, b.Data)
}
