package blockchain

import (
	"fmt"
	"gcoin/util"
	"slices"
	"strings"
	"time"
)

type Chain[T any] []Block[T]

func NewChain[T any](s []T) Chain[T] {
	var chain Chain[T]
	for _, data := range s {
		b := chain.NextUnmintedBlock(data)
		b.Mine()
		chain = append(chain, b)
	}
	return chain
}

func RebuildChain[T any](m map[util.Hash]Block[T], cur Block[T]) (Chain[T], error) {
	var buf []Block[T]
	for {
		buf = append(buf, cur)
		if cur.Index == 0 {
			break
		}
		prev, ok := m[cur.prevHash]
		if !ok {
			return nil, fmt.Errorf("no prev found among blocks for %#v", cur)
		}
		cur = prev
	}
	slices.Reverse(buf)
	return Chain[T](buf), nil
}

func (chain Chain[T]) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("cd=%d len=%d\n", chain.GetCumulativeDifficulty(), len(chain)))
	for _, b := range chain {
		builder.WriteString(fmt.Sprintf("%s\n", b))
	}
	return builder.String()
}

func (chain Chain[T]) NextUnmintedBlock(data T) Block[T] {
	last := util.Last(chain)
	if last == nil {
		return NewBlock(data)
	}
	b := Block[T]{
		Index:     uint64(last.Index + 1),
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
		prevHash:  last.hash,
		hash:      util.Hash{},
		Nonce:     0}
	b.Exp = chain.BlockExp(&b)
	b.CmDf = last.CmDf + (1 << b.Exp)
	return b
}

func (chain Chain[T]) Append(b Block[T]) (Chain[T], error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	last := util.Last(chain)
	if last == nil {
		if b.Index != 0 {
			return nil, fmt.Errorf("index mismatch")
		}
		return Chain[T]{b}, nil
	}

	if err := last.ValidateNextBlock(&b); err != nil {
		return nil, err
	}
	return append(chain, b), nil
}

func (chain Chain[T]) BlockExp(b *Block[T]) uint8 {
	last := util.Last(chain)
	if b.Index%NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT != 0 {
		return last.Exp
	}

	// expect b.index != 0
	timeTaken := b.Timestamp - chain[b.Index-NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT].Timestamp
	if timeTaken > TIME_EXPECTED*2 {
		if last.Exp > 0 {
			return last.Exp - 1
		} else {
			return 0
		}
	} else if timeTaken < TIME_EXPECTED/2 {
		return last.Exp + 1
	} else {
		return last.Exp
	}
}

func (chain Chain[T]) Validate() error {
	n := len(chain)
	if n == 0 {
		return fmt.Errorf("chain is empty")
	}
	for i := 0; i < n-1; i++ {
		a := &chain[i]
		b := &chain[i+1]
		if err := a.ValidateNextBlock(b); err != nil {
			return err
		}
	}
	return nil
}

func (chain Chain[T]) GetCumulativeDifficulty() uint64 {
	last := util.Last(chain)
	if last == nil {
		return 0
	}
	return last.CmDf
}

func (chain Chain[T]) Less(other Chain[T]) bool {
	return chain.GetCumulativeDifficulty() < other.GetCumulativeDifficulty()
}
