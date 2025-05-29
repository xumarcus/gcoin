package blockchain

import (
	"fmt"
	"gcoin/util"
	"slices"
	"time"
)

type Chain[T util.Hashable] []Block[T]

func NewChain[T util.Hashable](s []T) Chain[T] {
	var chain Chain[T]
	for _, data := range s {
		b := chain.NextUnmintedBlock(data)
		b.Mine()
		chain = append(chain, b)
	}
	return chain
}

func RebuildChain[T util.Hashable](m map[util.Hash]Block[T], cur Block[T]) (Chain[T], error) {
	var chain Chain[T]
	for {
		chain = append(chain, cur)
		if cur.BlockHeader.Index == 0 {
			break
		}
		prev, ok := m[cur.BlockHeader.PrevHash]
		if !ok {
			return nil, fmt.Errorf("prev not found")
		}
		cur = prev
	}
	slices.Reverse(chain)
	if err := chain.Validate(); err != nil {
		return nil, err
	}
	return chain, nil
}

func (chain Chain[T]) ComputeTarget(bh *BlockHeader) uint8 {
	index := bh.Index
	if index == 0 {
		return 0
	}

	prev := &chain[index-1].BlockHeader
	if index%NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT != 0 {
		return prev.Target
	}

	ancestor := &chain[index-NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT].BlockHeader
	timeTaken := bh.Timestamp - ancestor.Timestamp
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

func (chain Chain[T]) NextUnmintedBlock(data T) Block[T] {
	last := util.Last(chain)
	if last == nil {
		return NewBlock(data)
	}
	bh := &last.BlockHeader
	index := bh.Index + 1
	blockHeader := BlockHeader{
		Index:     index,
		InnerHash: data.Hash(),
		Nonce:     0,
		PrevHash:  bh.Hash(),
		Timestamp: time.Now().UnixMilli()}
	blockHeader.Target = chain.ComputeTarget(&blockHeader)
	blockHeader.Diff = bh.Diff + (1 << blockHeader.Target)
	return Block[T]{
		BlockHash:   blockHeader.Hash(),
		BlockHeader: blockHeader,
		Data:        data}
}

func (chain Chain[T]) ValidateNextBlock(b *Block[T]) error {
	if b.BlockHeader.Index != uint64(len(chain)) {
		return fmt.Errorf("index != len")
	}
	return chain.ValidateBlock(b)
}

func (chain Chain[T]) ValidateBlock(b *Block[T]) error {
	if err := b.Validate(); err != nil {
		return err
	}
	bh := &b.BlockHeader

	switch index := bh.Index; {
	case index > uint64(len(chain)):
		return fmt.Errorf("index too big")
	case index == 0:
		if !bh.isGenesisBlockHeader() {
			return fmt.Errorf("not isGenesisBlockHeader")
		}
	default:
		prev := &chain[index-1]
		if err := b.ValidateWithPrev(prev); err != nil {
			return err
		}
		if bh.Target != chain.ComputeTarget(bh) {
			return fmt.Errorf("target mismatch")
		}
	}
	return nil
}

func (chain Chain[T]) Validate() error {
	for i := range chain {
		if err := chain.ValidateBlock(&chain[i]); err != nil {
			return err
		}
	}
	return nil
}

func (chain Chain[T]) Difficulty() uint64 {
	last := util.Last(chain)
	if last == nil {
		return 0
	}
	return last.BlockHeader.Diff
}

func (chain Chain[T]) Less(other Chain[T]) bool {
	return chain.Difficulty() < other.Difficulty()
}
