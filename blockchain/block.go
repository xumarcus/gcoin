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

func (b *Block[T]) ValidateWithPrev(prev *Block[T]) error {
	if err := b.BlockHeader.ValidateWithPrev(&prev.BlockHeader); err != nil {
		return err
	}
	if b.BlockHeader.PrevHash != prev.BlockHash {
		return fmt.Errorf("prevHash mismatch")
	}
	return nil
}
