package blockchain

import (
	"fmt"
	"gcoin/util"
)

type Block[T util.Hashable] struct {
	BlockHash   util.Hash
	BlockHeader BlockHeader
	Data        T
}

func NewBlock[T util.Hashable](data T) Block[T] {
	innerHash := data.Hash()
	blockHeader := NewBlockHeader(innerHash)
	return Block[T]{
		BlockHash:   blockHeader.Hash(),
		BlockHeader: blockHeader,
		Data:        data}
}

func (b Block[T]) Equal(other Block[T]) bool {
	return b.BlockHash == other.BlockHash
}

func (b *Block[T]) Mine() {
	b.BlockHash = b.BlockHeader.Mine()
}

func (b *Block[T]) Validate() error {
	if err := b.BlockHeader.Validate(); err != nil {
		return err
	}
	if b.BlockHeader.InnerHash != b.Data.Hash() {
		return fmt.Errorf("InnerHash mismatch")
	}
	if b.BlockHeader.Hash() != b.BlockHash {
		return fmt.Errorf("hash mismatch")
	}
	return nil
}

func (b *Block[T]) ValidateSuccessor(succ *Block[T]) error {
	if err := b.BlockHeader.ValidateSuccessor(&succ.BlockHeader); err != nil {
		return err
	}
	if b.BlockHash != succ.BlockHeader.PrevHash {
		return fmt.Errorf("prevHash mismatch")
	}
	return nil
}
