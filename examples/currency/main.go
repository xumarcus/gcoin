package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
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

// Adjust TALLY_LEN if the program panics from OOB
const TALLY_LEN = 70      // Number of blocks to tally for reporting
const SIM_LEN = 40        // Max number of transfer from a wallet
const MAX_AMOUNT = 5      // Max amount involved per transfer
const TRANSACTION_FEE = 1 // How much fee is paid per transfer
const N = 4               // How many nodes on each of the two "sides" of the mesh

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

// prepareNextUnmintedBlock prepares the next block to be mined by:
// 1. Copying the current UTXO state
// 2. Validating and selecting transactions from mempool
// 3. Creating a new block with valid transactions
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
		utxoDb.UpdateTxData(&txn.TxData)
		txns = append(txns, txn)
	}

	address := node.wallet.GetAddress()
	bt := c.NewBlockTransactions(txns, address)
	return node.protected.chain.NextUnmintedBlock(bt)
}

// Mine should not hold the lock while it is mining the next block
func (node *Node) Mine() {
	b := node.prepareNextUnmintedBlock()

	// If a node mines blocks within the same millisecond, then the coinbase txIds can collide
	<-time.After(50 * time.Millisecond)
	select {
	case node.rMined <- b:
	case <-time.After(2 * time.Second):
		// do nothing
	}
}

// handleBlock processes an incoming block by:
// 1. Validating the block
// 2. Checking for duplicates
// 3. Rebuilding the chain if the block has higher difficulty
// Returns error if block is invalid or duplicate
func (node *Node) handleBlock(b c.Block) error {
	if err := b.Validate(); err != nil {
		panic(err)
	}

	_, ok := node.blocks[b.BlockHash]
	if ok {
		return fmt.Errorf("duplicate found")
	}
	node.blocks[b.BlockHash] = b

	node.mu.Lock()
	defer node.mu.Unlock()

	if b.BlockHeader.Diff <= node.protected.chain.Difficulty() {
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
 *
 * In this simulation we never purge transactions from our mempool
 */

// handleTransaction processes an incoming transaction by:
// 1. Validating the transaction
// 2. Checking for duplicates
// 3. Adding to mempool if valid
// Returns error if transaction is invalid or duplicate
func (node *Node) handleTransaction(txn c.RegularTransaction) error {
	if err := txn.Validate(); err != nil {
		panic(err)
	}

	txId := txn.TxId
	if node.txIds.Contains(txId) {
		return fmt.Errorf("duplicate found")
	}
	node.txIds.Add(txId)

	node.mu.Lock()
	defer node.mu.Unlock()

	node.protected.mempool = append(node.protected.mempool, txn)
	return nil
}

// handleMinedBlock processes a newly mined block by:
// 1. Appending it to the chain
// 2. Updating the UTXO database
// It does not try to rebuild the chain
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

// Relay handles incoming messages by:
// 1. Processing blocks, transactions or mined blocks
// 2. Broadcasting valid blocks or transactions to subscribers
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

func (node *Node) makeSimulatedTransaction(nodes []Node) (*c.RegularTransaction, error) {
	node.mu.Lock()
	defer node.mu.Unlock()

	address := node.wallet.GetAddress()
	if node.protected.utxoDb.AvailableFunds(address) <= MAX_AMOUNT {
		return nil, fmt.Errorf("%s out of funds", address)
	}

	recvNode := &nodes[node.rd.IntN(2*N)]
	amount := 1 + node.rd.Uint64N(MAX_AMOUNT)

	return node.wallet.MakeRegularTransaction(&node.protected.utxoDb, recvNode.wallet.GetAddress(), amount, TRANSACTION_FEE)
}

// Sim performs the node simulation by:
// 1. Waiting a random delay (100-350ms)
// 2. Creating a simulated transaction
// 3. Sending it to the transaction channel
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

// main initializes and runs a decentralized blockchain network simulation with 2*N nodes.
// Key characteristics of this simulation:
//
// 1. Consensus:
//   - Final blockchain states across nodes may differ due to:
//   - Natural blockchain forks during simulation
//   - Network latency in block propagation
//   - Variation in mining speeds between nodes
//   - However, chains should converge up to TALLY_LEN blocks due to:
//   - The longest valid chain rule
//   - Eventually consistent gossip protocol
//   - Automatic chain reorganization logic
//
// 2. Integrity:
//   - Messages are eventually delivered in FIFO order
//     but not necessarily causal, as the channels are buffered.
//   - Each node has to be validating and rebuild the chain upon reorg
//     to recover the causal order of events/
//
// 3. Network Topology:
//   - Nodes are connected in a mesh (but not complete, i.e., clique):
//   - Two groups of N nodes with cross-connections
//   - Each node connects to all nodes in the opposite group
//   - Message propagation requires at most 2 hops
//   - This models real-world P2P networks where:
//   - Not all nodes connect to each other directly
//   - Network partitions can occur temporarily
//   - Messages propagate through gossip protocol
//
// 4. Simulation Output:
//   - Outputs two sets of data for analysis:
//   - UTXO set summaries (for economic state)
//   - Full blockchain histories (for consensus analysis)
func main() {
	nodes := make([]Node, 2*N)
	for i := range nodes {
		node := &nodes[i]

		node.rBlock = make(chan c.Block, 1024768)
		node.rMined = make(chan c.Block) // Unbuffered
		node.rTxn = make(chan c.RegularTransaction, 1024768)
		node.ssBlock = make([]chan c.Block, N)
		node.ssTxn = make([]chan c.RegularTransaction, N)

		node.rd = *rand.New(rand.NewPCG(42, uint64(i)))
		node.wallet = c.NewWallet()
		node.txIds = mapset.NewSet[c.TxId]()
		node.blocks = make(map[util.Hash]c.Block)

		node.protected.utxoDb = c.NewUtxoDb()
	}

	// Mesh interconnect
	for i := range N {
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
			for range SIM_LEN {
				node.Sim(nodes)
			}
			node.isStop.Store(true) // Signal other routines to stop
		}()
	}
	wg.Wait()

	for i := range nodes {
		node := &nodes[i]
		chain := node.protected.chain[:TALLY_LEN]
		utxoDb := c.NewUtxoDbFromChain(chain)
		if data, err := json.MarshalIndent(utxoDb.Summary(), "", "\t"); err != nil {
			panic(err)
		} else {
			os.Stdout.Write(data)
		}
	}

	for i := range nodes {
		node := &nodes[i]
		if data, err := json.MarshalIndent(node.protected.chain, "", "\t"); err != nil {
			panic(err)
		} else {
			os.Stdout.Write(data)
		}
	}
}
