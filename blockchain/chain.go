package blockchain

import (
	"fmt"
	"gcoin/util"
	"slices"
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
	var blocks []Block[T]
	for {
		blocks = append(blocks, cur)
		if cur.BlockHeader.Index == 0 {
			break
		}
		prev, ok := m[cur.BlockHeader.PrevHash]
		if !ok {
			return nil, fmt.Errorf("prev not found")
		}
		cur = prev
	}
	slices.Reverse(blocks)
	return Chain[T](blocks), nil
}

func (chain Chain[T]) NextUnmintedBlock(data T) Block[T] {
	last := util.Last(chain)
	if last == nil {
		return NewBlock(data)
	}

	var ancestorTimestamp int64
	index := last.BlockHeader.Index + 1
	if index >= NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT {
		ancestorTimestamp = chain[index-NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT].BlockHeader.Timestamp
	}

	innerHash := data.Hash()
	blockHeader := last.BlockHeader.NextBlockHeader(innerHash, ancestorTimestamp)
	return Block[T]{
		BlockHash:   blockHeader.Hash(),
		BlockHeader: blockHeader,
		Data:        data}
}

func (chain Chain[T]) Append(b Block[T]) (Chain[T], error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	last := util.Last(chain)
	if last == nil {
		if b.BlockHeader.Index != 0 {
			return nil, fmt.Errorf("index mismatch")
		}
		return Chain[T]{b}, nil
	}

	if err := last.ValidateSuccessor(&b); err != nil {
		return nil, err
	}
	return append(chain, b), nil
}

func (chain Chain[T]) Validate() error {
	n := len(chain)
	if n == 0 {
		return nil
	}
	for i := 0; i < n-1; i++ {
		if err := chain[i].ValidateSuccessor(&chain[i+1]); err != nil {
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
