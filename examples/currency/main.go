package main

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"gcoin/blockchain"
	c "gcoin/currency"
	"gcoin/util"

	"github.com/jinzhu/copier"

	mapset "github.com/deckarep/golang-set/v2"
)

type Node struct {
	mu        sync.Mutex
	protected struct {
		chain   c.Chain
		utxoDb  c.UtxoDb
		mempool []c.RegularTransaction // Assume validated
	}
	txIds   mapset.Set[c.TxId]    // Exclusive to handleTransaction
	blocks  map[util.Hash]c.Block // Exclusive to handleBlock
	rd      rand.Rand
	rBlock  chan c.Block
	rMined  chan c.Block
	rTxn    chan c.RegularTransaction
	ssBlock []chan c.Block
	ssTxn   []chan c.RegularTransaction
	wallet  c.Wallet
	isStop  atomic.Bool
}

const SIMLEN = 40

func broadcast[T any](ss []chan T, data T) {
	var wg sync.WaitGroup
	for _, s := range ss {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case s <- data:
			case <-time.After(2 * time.Second):
				// do nothing
			}
		}()
	}
	wg.Wait()
}

func (node *Node) prepareNextUnmintedBlock() c.Block {
	node.mu.Lock()
	defer node.mu.Unlock()

	var utxoDb c.UtxoDb
	copier.Copy(&utxoDb, &node.protected.utxoDb)

	var txns []c.RegularTransaction
	for _, txn := range node.protected.mempool {
		if err := utxoDb.ValidateRegularTransaction(&txn); err != nil {
			continue
		}
		utxoDb.UpdateTransaction(&txn)
		txns = append(txns, txn)
	}

	address := node.wallet.GetAddress()
	bt := c.NewBlockTransactions(txns, address)
	return node.protected.chain.NextUnmintedBlock(bt)
}

func (node *Node) Mine() {
	b := node.prepareNextUnmintedBlock()
	b.Mine()
	select {
	case node.rMined <- b:
	case <-time.After(2 * time.Second):
		// do nothing
	}
}

func (node *Node) handleBlock(b c.Block) error {
	if err := b.Validate(); err != nil {
		panic(err)
	}

	_, ok := node.blocks[b.Hash]
	if ok {
		return fmt.Errorf("duplicate found")
	}
	node.blocks[b.Hash] = b

	node.mu.Lock()
	defer node.mu.Unlock()

	if b.Cd <= node.protected.chain.GetCumulativeDifficulty() {
		return nil
	}

	chain, err := blockchain.RebuildChain(node.blocks, b)
	if err != nil {
		panic(err)
	}

	node.protected.chain = chain
	node.protected.utxoDb = c.NewUtxoDbFromChain(chain)
	return nil
}

func (node *Node) handleTransaction(txn c.RegularTransaction) error {
	if err := txn.Validate(); err != nil {
		panic(err)
	}

	if node.txIds.Contains(txn.TxId()) {
		return fmt.Errorf("duplicate found")
	}
	node.txIds.Add(txn.TxId())

	node.mu.Lock()
	defer node.mu.Unlock()

	node.protected.mempool = append(node.protected.mempool, txn)
	return nil
}

func (node *Node) handleMinedBlock(b c.Block) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	chain, err := node.protected.chain.Append(b)
	if err != nil {
		return err
	}

	node.protected.chain = chain
	node.protected.utxoDb.UpdateFromBlockTransactions(&b.Data)
	return nil
}

// To save message bandwidth we transmit blocks
func (node *Node) Relay() {
	select {
	case b := <-node.rBlock:
		if err := node.handleBlock(b); err == nil {
			broadcast(node.ssBlock, b)
		}
	case txn := <-node.rTxn:
		if err := node.handleTransaction(txn); err == nil {
			broadcast(node.ssTxn, txn)
		}
	case b := <-node.rMined:
		if err := node.handleMinedBlock(b); err == nil {
			broadcast(node.ssBlock, b)
		}
	case <-time.After(2 * time.Second):
		// do nothing
	}
}

const MAX_AMOUNT = 5

func (node *Node) makeSimulatedTransaction(nodes []Node) (*c.RegularTransaction, error) {
	node.mu.Lock()
	defer node.mu.Unlock()

	address := node.wallet.GetAddress()
	if node.protected.utxoDb.AvailableFunds(address) <= MAX_AMOUNT {
		return nil, fmt.Errorf("%s out of funds", address)
	}

	recvNode := &nodes[node.rd.IntN(2*N)]
	amount := 1 + node.rd.Uint64N(MAX_AMOUNT)

	return node.wallet.MakeRegularTransaction(&node.protected.utxoDb, recvNode.wallet.GetAddress(), amount)
}

func (node *Node) Sim(nodes []Node) {
	ms := 100 + node.rd.IntN(250)
	<-time.After(time.Duration(ms) * time.Millisecond)

	txn, err := node.makeSimulatedTransaction(nodes)
	if err != nil {
		return
	}

	select {
	case node.rTxn <- *txn:
	case <-time.After(2 * time.Second):
		// do nothing
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

const N = 4

func main() {
	nodes := make([]Node, 2*N)
	for i := range nodes {
		node := &nodes[i]
		node.rd = *rand.New(rand.NewPCG(42, uint64(i)))
		node.rBlock = make(chan c.Block, 1024768)
		node.rMined = make(chan c.Block)
		node.rTxn = make(chan c.RegularTransaction, 1024768)
		node.ssBlock = make([]chan c.Block, N)
		node.ssTxn = make([]chan c.RegularTransaction, N)
		node.wallet = c.NewWallet()

		node.txIds = mapset.NewSet[c.TxId]()
		node.blocks = make(map[util.Hash]c.Block)

		node.protected.utxoDb = c.NewUtxoDb()
	}

	// Mesh interconnect
	for i := 0; i < N; i++ {
		a := &nodes[i]
		b := &nodes[i+N]
		for j := 0; j < N; j++ {
			a.ssBlock[j] = nodes[j+N].rBlock
			b.ssBlock[j] = nodes[j].rBlock
			a.ssTxn[j] = nodes[j+N].rTxn
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
				if node.isStop.Load() {
					break
				}
				node.Relay()
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if node.isStop.Load() {
					break
				}
				node.Mine()
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for range SIMLEN {
				node.Sim(nodes)
			}
			node.isStop.Store(true)
		}()
	}
	wg.Wait()

	for i := range nodes {
		node := &nodes[i]

		fmt.Println("---")
		for j := range nodes {
			x := &nodes[j]
			address := x.wallet.GetAddress()
			funds := node.protected.utxoDb.AvailableFunds(address)
			fmt.Printf("%s: $%d\n", address, funds)
		}
	}

	for i := range nodes {
		node := &nodes[i]
		fmt.Println("---")
		fmt.Println(node.protected.chain)
	}
}
