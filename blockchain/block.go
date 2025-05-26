package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"gcoin/util"
	"time"
)

type Block[T any] struct {
	Data      T
	index     uint64
	timestamp int64
	prevHash  util.Hash
	Hash      util.Hash
	exp       uint8
	Cd        uint64
	nonce     uint64
}

func NewBlock[T any](data T) Block[T] {
	b := Block[T]{
		Data:      data,
		index:     0,
		timestamp: time.Now().UnixMilli(),
		prevHash:  util.Hash{},
		Hash:      util.Hash{},
		exp:       0,
		Cd:        1,
		nonce:     0}
	b.Hash = b.ComputeHash()
	return b
}

func (b Block[T]) Equal(other Block[T]) bool {
	return b.Hash == other.Hash
}

func (b *Block[T]) ComputeHash() util.Hash {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, b.index)
	binary.Write(&buf, binary.BigEndian, b.timestamp)
	binary.Write(&buf, binary.BigEndian, b.Data)
	binary.Write(&buf, binary.BigEndian, b.prevHash)
	binary.Write(&buf, binary.BigEndian, b.exp)
	binary.Write(&buf, binary.BigEndian, b.Cd)
	binary.Write(&buf, binary.BigEndian, b.nonce)
	return sha256.Sum256(buf.Bytes())
}

func (b *Block[T]) Mine() {
	for {
		b.Hash = b.ComputeHash()
		if uint8(b.Hash.LeadingZeros()) < b.exp {
			b.nonce++
		} else {
			return
		}
	}
}

func (b *Block[T]) Validate() error {
	if b.ComputeHash() != b.Hash {
		return fmt.Errorf("hash mismatch")
	}
	if b.timestamp-60 >= time.Now().UnixMilli() {
		return fmt.Errorf("is from future")
	}
	return nil
}

func (a *Block[T]) ValidateNextBlock(b *Block[T]) error {
	if b.index != a.index+1 {
		return fmt.Errorf("index mismatch")
	}
	if b.prevHash != a.Hash {
		return fmt.Errorf("prevHash mismatch")
	}
	if b.Cd != a.Cd+(1<<b.exp) {
		return fmt.Errorf("cd mismatch")
	}
	if b.timestamp <= a.timestamp-60 {
		return fmt.Errorf("is from past")
	}
	return nil
}

func (b Block[T]) String() string {
	return fmt.Sprintf("%d:(exp=%d) %v", b.index, b.exp, b.Data)
}
