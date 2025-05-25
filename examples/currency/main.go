package main

import (
	"gcoin/internal/gcoin"
	"math/rand/v2"
	"slices"
	"sync"
	"time"
)

type Block = gcoin.Block[[]gcoin.Transaction]

type Node struct {
	ledger  gcoin.Ledger
	mempool []gcoin.Transaction
	rd      rand.Rand
	rBlock  chan Block
	rMined  chan Block
	rTxn    chan gcoin.Transaction
	ssBlock []chan Block
	ssTxn   []chan gcoin.Transaction
	blocks  []Block
	wallet  gcoin.Wallet
}

func (node *Node) Mine() {
	const DEFAULT_COINBASE_AMOUNT = 50

	coinbase := uint64(DEFAULT_COINBASE_AMOUNT)
	var txns []gcoin.Transaction
	for _, txn := range node.mempool {
		fee, err := node.ledger.ValidateAndComputeTransactionFee(&txn)
		if err == nil {
			coinbase += fee
			txns = append(txns, txn)
		}
	}

	coinbaseTxn := node.wallet.MakeCoinbaseTransaction(coinbase)
	txns = append([]gcoin.Transaction{coinbaseTxn}, txns...)

	b := node.ledger.GetChain().NextBlock(txns)
	b.Mine()
	node.rMined <- b
}

func main() {
	n := 4

	nodes := make([]Node, 2*n)
	for i := range nodes {
		node := &nodes[i]
		node.rd = *rand.New(rand.NewPCG(42, uint64(i)))
		node.rBlock = make(chan Block)
		node.rMined = make(chan Block)
		node.rTxn = make(chan gcoin.Transaction)
		node.ssBlock = make([]chan Block, n)
		node.ssTxn = make([]chan gcoin.Transaction, n)
		node.wallet = gcoin.NewWallet()
	}

	for i := 0; i < n; i++ {
		a := &nodes[i]
		b := &nodes[i+n]
		for j := 0; j < n; j++ {
			a.ssBlock[j] = nodes[j+n].rBlock
			b.ssBlock[j] = nodes[j].rBlock
			a.ssTxn[j] = nodes[j+n].rTxn
			b.ssTxn[j] = nodes[j].rTxn
		}
	}

	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		node := &nodes[i]
		go func() {
			defer wg.Done()

			for {
				// To save message bandwidth we transmit blocks instead of the entire chain
				select {
				case b := <-node.rBlock:
					if err := b.Validate(); err != nil {
						panic(err)
					}
					eq := func(other Block) bool {
						return b.Equal(&other)
					}
					if slices.ContainsFunc(node.blocks, eq) {
						continue
					}
					node.blocks = append(node.blocks, b)
					chain, err := gcoin.MakeChainFromBlocks(node.blocks)
					if err != nil {
						panic(err)
					}
					node.ledger = gcoin.MakeLedgerFromChain(chain)
					/*
					 * Mempool Management Notes:
					 *
					 * 1. Chain-State Consistency:
					 *    - The mempool may contain transactions that became invalid after ledger updates
					 *    - We cannot proactively purge these because:
					 *      a) Reorgs may make them valid again
					 *      b) Validation is expensive to recompute continuously
					 *
					 * 2. Real-World Mempool Constraints:
					 *    - Retention: Transactions expire after ~14 days (time or block-based)
					 *    - Capacity: Default 300MB pool (evicts lowest fee-rate tx when full)
					 *    - Replacement: Only allowed for RBF-enabled transactions
					 *
					 * 3. Transaction Security:
					 *    - To prevent malicious replays of stale transactions:
					 *      a) Senders should use nSequence-based RBF to invalidate old versions
					 *      b) Alternatively, spend the same UTXOs in a new transaction
					 *
					 * 4. Confirmation Acceleration:
					 *    - For legitimate stuck transactions:
					 *      a) CPFP: Attach high-fee child transaction
					 *      b) Direct replacement: Increase fee with RBF
					 */
					// Do nothing with the mempool
				case txn := <-node.rTxn:
					if err := txn.Validate(); err != nil {
						panic(err)
					}
					eq := func(other gcoin.Transaction) bool {
						return txn.Equal(&other)
					}
					if slices.ContainsFunc(node.mempool, eq) {
						continue
					}
					node.mempool = append(node.mempool, txn)
				case b := <-node.rMined:
					// TODO parallelize
					for _, sBlock := range node.ssBlock {
						sBlock <- b
					}
					go node.Mine()
				default:
					go node.Mine()
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				<-time.After(250 * time.Millisecond)
				// generate txn to rTxn
			}
		}()
	}
	wg.Wait()
}
