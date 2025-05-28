# Tutorial
In this tutorial we will code from scratch some of the basic concepts that are needed for a working cryptocurrency.

Unlike [Naivecoin](https://lhartikk.github.io/) we will code in Go. It is statically typed and performant. Most importantly, we can model communication with Go channels instead of WebSockets. Whereas Websockets operate over TCP/IP, adding network stack overhead (headers, latency, connection management) and consuming significant OS resources, Go channels are lightweight and allow large-scale, high-performance simulations.

## Blockchain

### Introduction
A blockchain is a distributed, fault-tolerant, append-only, trustless, and permissionless database.
- Distributed: The database is maintained across multiple nodes (computers) in a peer-to-peer network, rather than being stored in a central location. No single entity has full control.
- Append-only: Data can only be added (in blocks), not modified or deleted.
- Trustless: Consensus mechanisms (e.g., Proof of Work) validate data without intermediaries.
- Permissionless: Anyone can join the network, participate in validation, and submit data.
- Fault tolerant: The system continues functioning even if some nodes fail or act maliciously (i.e. Byzantine fault).

### Block header
Let us first define a few structs.
```go
type Hash [32]byte

type Hashable interface {
	Hash() Hash
}

type Block[T util.Hashable] struct {
	BlockHash   util.Hash   // BlockHash == BlockHeader.Hash()
	BlockHeader BlockHeader
	Data        T
}

type BlockHeader struct {
	Diff      uint64    // Sum of block difficulties
	Index     uint64    // chain[Index] == Block
	InnerHash util.Hash // Data.Hash() == BlockHeader.InnerHash
	Nonce     uint64    // The "Proof of work"
	PrevHash  util.Hash // BlockHash of the previous block
	Target    uint8     // Measures the difficulty of the proof
	Timestamp int64     // When is the block created
}

type Chain[T util.Hashable] []Block[T]
```

The block hash is calculated over the entire block header. `NewHash` encodes all the fields onto a buffer. Then the bytes inside the buffer is hashed.

It is important that `PrevHash` is included in the block header for tamper resistance. Notice that when an attacker tries to modify a block, all the hashes of subsequent blocks will change: the deeper this block is, the more blocks the attacker has to modify.

```go
func (bh *BlockHeader) Hash() util.Hash {
	return util.NewHash(bh)
}

func NewHash[T any](data T) Hash {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		panic(err)
	}
	return sha256.Sum256(buf.Bytes())
}
```

When we generate a block, we will need the hash of the preceding block header `bh`, as well as the hash of the current block's data `innerHash`. (known as `merkleRoot` IRL) 

```go
func (bh *BlockHeader) NextBlockHeader(innerHash util.Hash, ancestorTimestamp int64) BlockHeader {
	cur := BlockHeader{
		Index:     bh.Index + 1,
		InnerHash: innerHash,
		Nonce:     0,
		PrevHash:  bh.Hash(),
		Timestamp: time.Now().UnixMilli()}
	
	// ...

	return cur
}

func (chain Chain[T]) NextUnmintedBlock(data T) Block[T] {
	last := util.Last(chain)
	if last == nil {
		return NewBlock(data)
	}

	// ...

	innerHash := data.Hash()
	blockHeader := last.BlockHeader.NextBlockHeader(innerHash, ancestorTimestamp)
	return Block[T]{
		BlockHash:   blockHeader.Hash(),
		BlockHeader: blockHeader,
		Data:        data}
}
```

As other nodes send blocks to us, we will need to validate if those blocks and their content are consistent. Hence all structs have to implement this `Validated` interface.
```go
type Validated interface {
	Validate() error
}
```

In distributed systems it is impossible to ensure all clocks are perfectly in sync. Here we tolerate a difference of up to `NUM_MILLISECONDS_TIME_DIFF_TOLERANCE`.
```go
func (bh *BlockHeader) Validate() error {
	if bh.Timestamp-NUM_MILLISECONDS_TIME_DIFF_TOLERANCE >= time.Now().UnixMilli() {
		return fmt.Errorf("future block time Rule")
	}
	return nil
}

func (bh *BlockHeader) ValidateSuccessor(succ *BlockHeader) error {
	if succ.Index != bh.Index+1 {
		return fmt.Errorf("index mismatch")
	}
	/*
	if succ.Diff != bh.Diff+(1<<succ.Target) {
		return fmt.Errorf("diff mismatch")
	}
	*/
	if succ.Timestamp <= bh.Timestamp-NUM_MILLISECONDS_TIME_DIFF_TOLERANCE {
		return fmt.Errorf("past time rule")
	}
	return nil
}
```

With this we can validate if the chain is consistent. (See code for full implementation)

```go
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

func (b *Block[T]) ValidateSuccessor(succ *Block[T]) error {
	if err := b.BlockHeader.ValidateSuccessor(&succ.BlockHeader); err != nil {
		return err
	}
	if b.BlockHash != succ.BlockHeader.PrevHash {
		return fmt.Errorf("prevHash mismatch")
	}
	return nil
}
```

### Sybil-resistance
To achieve permissionless distributed consensus, the network must impose Sybil-resistant mechanisms that restrict an attacker’s ability to flood the system with malicious nodes. Without such safeguards, a bad actor could easily dominate the network by spawning fake identities.

Bitcoin solves this problem with Proof of Work (PoW), which requires nodes to expend computational effort (hashing) to participate in consensus. This builds on HashCash, an early anti-spam system for email, where senders had to compute a cryptographic puzzle to prove legitimacy. Bitcoin extends this idea with:

- Dynamic difficulty adjustment: Automatically tunes the PoW challenge to maintain a consistent block time, regardless of network hash power.

- Economic incentives: Rewards (block subsidies + fees) motivate honest participation, while the cost of attacking (hardware + electricity) discourages malice.

Ethereum later introduced Proof of Stake (PoS), which replaces energy-intensive mining with capital-at-risk staking. Both PoW and PoS ensure that creating a node isn’t free, making large-scale attacks economically impractical.

Note: without transactions (and thus fees or rewards), there is no built-in economic incentive for nodes to participate honestly. In such a system, right now the blockchain’s integrity relies entirely on altruistic nodes: participants who follow the rules without personal gain.

### Proof of work

The proof of work is the `Nonce` such that the number of leading zeros in `BlockHash` meets the `Target`. Assuming that the hash function (SHA256) is a one-way trapdoor function, the easiest way to find this is to keep incrementing `Nonce`.

```go
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
```

### Dynamic difficulty adjustment
To maintain a healthy blockchain, new blocks must be added at a steady, predictable rate.
- If the interval between blocks is too short, then network latency becomes a major factor. The most recent miner gains an unfair advantage because their block propagates faster, increasing the risk of temporary forks (orphaned blocks).
- If the interval is too long, the chain updates too slowly, harming usability

To keep block production consistent despite fluctuations in network hash power, we need a mechanism to dynamically adjust the current `Target`. This is based on how much `timeTaken` to mine `NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT` blocks on the network.
```go
func nextTarget(prev *BlockHeader, cur *BlockHeader, ancestorTimestamp int64) uint8 {
	if cur.Index%NUM_BLOCKS_BETWEEN_DIFFICULTY_ADJUSTMENT != 0 {
		return prev.Target
	}

	timeTaken := cur.Timestamp - ancestorTimestamp
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
```

Note that an attacker can manipulate his timestamp to influence the computed `Target` for all the other nodes. The `NUM_MILLISECONDS_TIME_DIFF_TOLERANCE` scheme ensures that time is *mostly* monotonic, but **does not** counter this manipulation. This is important when we introduce transactions and mining rewards later, as the attacker can force the `Target` to drop in order to capture more mining rewards. This is left as exercise to the reader. (Future block past time rule)

### Chain resolution
With proof of work we can quantify the resources commited to a chain. This is but `Diff` of the last block on the chain, the sum of `2^Block.Target` for each `Block` on the chain.

```go
func (bh *BlockHeader) NextBlockHeader(innerHash util.Hash, ancestorTimestamp int64) BlockHeader {
	cur := BlockHeader { /* ... */ }
	cur.Target = nextTarget(bh, &cur, ancestorTimestamp)
	cur.Diff = bh.Diff + (1 << cur.Target)
	return cur
}
```

When a node observes two competing blockchain forks, it follows a simple rule: switch to the chain with the most accumulated proof-of-work (highest total `Difficulty()`). This ensures the network converges on a single canonical chain. Once we introduce mining rewards, a rational miner that maximizes rewards will switch as blocks on weaker forks risk being orphaned (no reward). This situation is also known as a Nash equilibrium.
```go
func (node *Node) handleBlock(b c.Block) error {
	if b.BlockHeader.Diff <= node.protected.chain.Difficulty() {
		return nil
	}

	chain, err := blockchain.RebuildChain(node.blocks, b)
	if err != nil {
		panic(err)
	}

	node.protected.chain = chain
}
```

## Cryptocurrency