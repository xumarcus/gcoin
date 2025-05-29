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

func (bh *BlockHeader) isGenesisBlockHeader() bool {
	return bh.Index == 0 && bh.Diff == 1 && bh.Nonce == 0 && bh.PrevHash == util.Hash{} && bh.Target == 0
}

func (bh *BlockHeader) Validate() error {
	if bh.Timestamp-NUM_MILLISECONDS_TIME_DIFF_TOLERANCE >= time.Now().UnixMilli() {
		return fmt.Errorf("is from far future")
	}
	return nil
}

func (bh *BlockHeader) ValidateWithPrev(prev *BlockHeader) error {
	if bh.Index != prev.Index+1 {
		return fmt.Errorf("index mismatch")
	}
	if bh.Diff != prev.Diff+(1<<bh.Target) {
		return fmt.Errorf("diff mismatch")
	}
	if bh.Timestamp <= prev.Timestamp-NUM_MILLISECONDS_TIME_DIFF_TOLERANCE {
		return fmt.Errorf("past time rule")
	}
	return nil
}
