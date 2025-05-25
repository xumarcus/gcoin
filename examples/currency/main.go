package main

import (
	"fmt"
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
	mu      sync.Mutex // guard (ledger, mempool)
	rd      rand.Rand
	rBlock  chan Block
	rMined  chan Block
	rTxn    chan gcoin.Transaction
	ssBlock []chan Block
	ssTxn   []chan gcoin.Transaction
	blocks  []Block
	wallet  gcoin.Wallet
}

const SIMLEN = 40

func (node *Node) Mine() bool {
	const DEFAULT_COINBASE_AMOUNT = 50

	n := len(node.ledger.GetChain())
	if n > SIMLEN {
		return false // Force stop
	}

	node.mu.Lock()
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
	txns = append([]gcoin.Transaction{coinbaseTxn}, txns...)l
	b := node.ledger.GetChain().NextBlock(txns)

	node.mu.Unlock()
	b.Mine()
	select {
	case node.rMined <- b:
		return true
	case <-time.After(5 * time.Second):
		return false // Relay() dies
	}
}

// To save message bandwidth we transmit blocks
func (node *Node) Relay() bool {
	select {
	case b := <-node.rBlock:
		if err := b.Validate(); err != nil {
			panic(err)
		}
		eq := func(other Block) bool {
			return b.Equal(other)
		}
		if slices.ContainsFunc(node.blocks, eq) {
			return true
		}
		node.blocks = append(node.blocks, b)

		if chain, err := gcoin.MakeChainFromBlocks(node.blocks); err != nil {
			panic(err)
		} else {
			node.mu.Lock()
			node.ledger = gcoin.MakeLedgerFromChain(chain)
			node.mu.Unlock()
		}

		// TODO parallelize
		for _, sBlock := range node.ssBlock {
			sBlock <- b
		}

		// Do nothing with the mempool
		return true
	case txn := <-node.rTxn:
		if err := txn.Validate(); err != nil {
			panic(err)
		}
		eq := func(other gcoin.Transaction) bool {
			return txn.Equal(other)
		}
		if slices.ContainsFunc(node.mempool, eq) {
			return true
		}
		node.mu.Lock()
		node.mempool = append(node.mempool, txn)
		node.mu.Unlock()

		// TODO parallelize
		for _, sTxn := range node.ssTxn {
			sTxn <- txn
		}

		return true
	case b := <-node.rMined:
		// TODO parallelize
		for _, sBlock := range node.ssBlock {
			sBlock <- b
		}

		chain := node.ledger.GetChain()
		chain = append(chain, b)
		node.mu.Lock()
		node.ledger = gcoin.MakeLedgerFromChain(chain)
		node.mu.Unlock()

		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

func (node *Node) SimulateTransfer(nodes []Node, n int, maxAmount uint64) {
	ms := 100 + node.rd.IntN(250)
	<-time.After(time.Duration(ms) * time.Millisecond)
	funds := node.wallet.GetAvailableFunds(&node.ledger)
	if funds <= maxAmount {
		return
	}
	receiverNode := &nodes[node.rd.IntN(2*n)]
	amount := uint64(1 + node.rd.IntN(int(maxAmount)))
	txn, err := node.wallet.MakeTransaction(&node.ledger, receiverNode.wallet.GetAddress(), amount)
	if err != nil {
		panic(err)
	}
	select {
	case node.rTxn <- txn:
	case <-time.After(5 * time.Second):
		return // Relay() dies
	}
}

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

func main() {
	n := 4
	nodes := make([]Node, 2*n)
	for i := range nodes {
		node := &nodes[i]
		node.rd = *rand.New(rand.NewPCG(42, uint64(i)))
		node.rBlock = make(chan Block, n*SIMLEN)
		node.rMined = make(chan Block)
		node.rTxn = make(chan gcoin.Transaction, n*SIMLEN)
		node.ssBlock = make([]chan Block, n)
		node.ssTxn = make([]chan gcoin.Transaction, n)
		node.wallet = gcoin.NewWallet()
	}

	// Mesh interconnect
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
		node := &nodes[i]

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if !node.Relay() {
					break
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if !node.Mine() {
					break
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for range SIMLEN {
				node.SimulateTransfer(nodes, n, 5)
			}
		}()
	}
	wg.Wait()

	for i := range nodes {
		node := &nodes[i]
		fmt.Println(node.ledger.GetChain())
	}
}
