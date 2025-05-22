package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/bits"
	"time"
)

// https://lhartikk.github.io/

type Block[T any] struct {
	index        uint64
	timestamp    int64
	data         T
	previousHash [32]byte
	hash         [32]byte
	difficulty   uint8
	nonce        uint64
}

type Chain[T any] []Block[T]

func computeHash[T any](b *Block[T]) [32]byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, b.index)
	binary.Write(&buf, binary.BigEndian, b.timestamp)
	binary.Write(&buf, binary.BigEndian, b.data)
	binary.Write(&buf, binary.BigEndian, b.previousHash)
	binary.Write(&buf, binary.BigEndian, b.difficulty)
	binary.Write(&buf, binary.BigEndian, b.nonce)
	return sha256.Sum256(buf.Bytes())
}

func NewBlock[T any](data T) Block[T] {
	b := Block[T]{
		index:        0,
		timestamp:    time.Now().UnixMilli(),
		data:         data,
		previousHash: [32]byte{},
		hash:         [32]byte{},
		difficulty:   0,
		nonce:        0}
	b.hash = computeHash(&b)
	return b
}

func numLeadingZeros(hash *[32]byte) int {
	ans := 0
	for _, x := range hash {
		ans += bits.LeadingZeros8(x)
		if x != 0 {
			break
		}
	}
	return ans
}

func NextBlock[T any](chain Chain[T], data T) Block[T] {
	last := chain[len(chain)-1]
	b := Block[T]{
		index:        last.index + 1,
		timestamp:    time.Now().UnixMilli(),
		data:         data,
		previousHash: last.hash,
		hash:         [32]byte{},
		difficulty:   0,
		nonce:        0}
	b.difficulty = blockDifficulty(chain, &b)
	for {
		b.hash = computeHash(&b)
		if uint8(numLeadingZeros(&b.hash)) < b.difficulty {
			b.nonce++
		} else {
			return b
		}
	}
}

func NewChain[T any](datas []T) Chain[T] {
	n := len(datas)
	chain := make([]Block[T], n)
	chain[0] = NewBlock(datas[0])
	for i := 1; i < n; i++ {
		chain[i] = NextBlock(Chain[T](chain[:i]), datas[i])
	}
	return chain
}

const NUM_MILLISECONDS_PER_BLOCK_GENERATED = 200
const NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT = 4
const TIME_EXPECTED = NUM_MILLISECONDS_PER_BLOCK_GENERATED * NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT

func blockDifficulty[T any](chain Chain[T], b *Block[T]) uint8 {
	last := chain[len(chain)-1]
	if b.index%NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT != 0 {
		return last.difficulty
	}

	// expect b.index != 0
	timeTaken := b.timestamp - chain[b.index-NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT].timestamp
	if timeTaken > TIME_EXPECTED*2 {
		if last.difficulty > 0 {
			return last.difficulty - 1
		} else {
			return 0
		}
	} else if timeTaken < TIME_EXPECTED/2 {
		return last.difficulty + 1
	} else {
		return last.difficulty
	}
}

func validate[T any](chain Chain[T]) error {
	for i := range chain {
		b := &chain[i]
		if uint64(i) != b.index {
			return fmt.Errorf("index")
		}
		if i != 0 {
			if chain[i-1].hash != b.previousHash {
				return fmt.Errorf("previousHash does not match prev")
			}
			if chain[i-1].timestamp-60 >= b.timestamp {
				return fmt.Errorf("time travel to the past")
			}
			if b.timestamp-60 >= time.Now().Unix() {
				return fmt.Errorf("time travel to the future")
			}
		}
		if computeHash(b) != b.hash {
			return fmt.Errorf("hash does not match block")
		}
	}
	return nil
}

func cumulativeDifficulty[T any](chain Chain[T]) int {
	ans := 0
	for i := range chain {
		b := &chain[i]
		ans += 1 << b.difficulty
	}
	return ans
}

func (chain Chain[T]) Less(other Chain[T]) bool {
	return cumulativeDifficulty(chain) < cumulativeDifficulty(other)
}
