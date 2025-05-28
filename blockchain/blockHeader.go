package blockchain

import (
	"fmt"
	"gcoin/util"
	"time"
)

type BlockHeader struct {
	Diff      uint64    // Sum of block difficulties
	Index     uint64    // chain[Index] == Block
	InnerHash util.Hash // Data.Hash() == BlockHeader.InnerHash
	Nonce     uint64    // The "Proof of work"
	PrevHash  util.Hash // BlockHash of the previous block
	Target    uint8     // Measures the difficulty of the proof
	Timestamp int64     // When is the block created
}

func NewBlockHeader(innerHash util.Hash) BlockHeader {
	return BlockHeader{
		Diff:      1,
		Index:     0,
		InnerHash: innerHash,
		Nonce:     0,
		PrevHash:  util.Hash{},
		Target:    0,
		Timestamp: time.Now().UnixMilli()}
}

func nextTarget(prev *BlockHeader, cur *BlockHeader, ancestorTimestamp int64) uint8 {
	if cur.Index%NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT != 0 {
		return prev.Target
	}

	timeTaken := cur.Timestamp - ancestorTimestamp
	if timeTaken > TIME_EXPECTED*2 {
		if prev.Target == 0 {
			return 0
		}
		return prev.Target - 1
	} else if timeTaken < TIME_EXPECTED/2 {
		return prev.Target + 1
	} else {
		return prev.Target
	}
}

func (bh *BlockHeader) NextBlockHeader(innerHash util.Hash, ancestorTimestamp int64) BlockHeader {
	cur := BlockHeader{
		Index:     bh.Index + 1,
		InnerHash: innerHash,
		Nonce:     0,
		PrevHash:  bh.Hash(),
		Timestamp: time.Now().UnixMilli()}
	cur.Target = nextTarget(bh, &cur, ancestorTimestamp)
	cur.Diff = bh.Diff + (1 << cur.Target)
	return cur
}

func (bh *BlockHeader) Hash() util.Hash {
	return util.NewHash(bh)
}

func (bh *BlockHeader) Mine() util.Hash {
	for {
		hash := bh.Hash()
		lz := uint8(hash.LeadingZeros())
		if lz >= bh.Target {
			return hash
		}
		bh.Nonce++
	}
}

func (bh *BlockHeader) Validate() error {
	if bh.Timestamp-NUM_MILLISECONDS_TIME_DIFF_TOLERANCE >= time.Now().UnixMilli() {
		return fmt.Errorf("future block time Rule")
	}
	return nil
}

func (bh *BlockHeader) ValidateSuccessor(succ *BlockHeader) error {
	if succ.Index != bh.Index+1 {
		return fmt.Errorf("index mismatch")
	}
	if succ.Diff != bh.Diff+(1<<succ.Target) {
		return fmt.Errorf("diff mismatch")
	}
	if succ.Timestamp <= bh.Timestamp-NUM_MILLISECONDS_TIME_DIFF_TOLERANCE {
		return fmt.Errorf("past time rule")
	}
	return nil
}
