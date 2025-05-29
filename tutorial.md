# Tutorial
In this tutorial we will code from scratch some of the basic concepts that are needed for a working cryptocurrency. The full project is under 1K LoC.

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

The block hash is calculated over the entire block header. `NewHash` encodes all the fields onto a buffer. Then the bytes inside the buffer is hashed. It is important that `PrevHash` is included among the fields for tamper resistance: when an attacker tries to modify a block, `BlockHash` of all the subsequent blocks will change; and the deeper this block is, the more blocks the attacker has to fix.

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

When we generate a block, we will need the hash of the preceding block header `bh`, as well as the hash of the current block's data `innerHash`.

In actual cryptocurrency implementations, we can put the root of a binary hash tree, known as a `merkleRoot`, in place of `innerHash`, for faster transaction verification. See [here](https://academy.binance.com/en/articles/merkle-trees-and-merkle-roots-explained) for more details.

```go
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
	/* See below for explanation
	blockHeader.Target = chain.ComputeTarget(&blockHeader)
	blockHeader.Diff = bh.Diff + (1 << blockHeader.Target)
	*/
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
		return fmt.Errorf("is from far future")
	}
	return nil
}
```

With this we can validate if the chain is consistent. (See code for full implementation)

```go
func (chain Chain[T]) Validate() error {
	for i := range chain {
		if err := chain.ValidateBlock(&chain[i]); err != nil {
			return err
		}
	}
	return nil
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
```

### Sybil resistance
To achieve permissionless distributed consensus, the network must impose Sybil-resistant mechanisms that restrict an attacker’s ability to flood the system with malicious nodes. Without such safeguards, a bad actor could easily dominate the network by spawning many malicious nodes.

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
```

Note that an attacker can manipulate his timestamp to influence the computed `Target` for all the other nodes. This is important when we introduce transactions and mining rewards later, as the attacker can force the `Target` to drop in order to capture more mining rewards. This is left as exercise to the reader. (Future block past time rule)

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

When a node observes two competing blockchain forks, it follows a simple rule: switch to the chain with the most accumulated proof-of-work (highest total `Difficulty()`). This ensures the network converges on a single canonical chain. Once we introduce mining rewards, a rational miner that maximizes rewards will switch as blocks on weaker forks risk being orphaned (no reward).
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
Check out the package `blockchain` for the full [code](blockchain).

## Cryptocurrency

### Transactions
A transaction involves transferring a cheque from one address to another. Here, an address is a hash of a public key, not the key itself. This adds an extra layer of security: even if an attacker can derive private keys from public keys, they must first reverse the hash to recover the public key, making the process more difficult.

In addition we can assign a unique `TxId` to every transaction, by hashing the contents in `TxData`.
```go
type Address = util.Hash
type TxId = util.Hash

type RegularTransaction struct {
	TransactionFee uint64
	TxId           TxId
	TxData         TxData
	Witness        Witness
}
```
To prevent forgery, the sender must digitally sign the transaction. This involves:
- Proving ownership: The sender provides their public key to demonstrate control over the address funding the transaction.
- Enabling verification: Others can use this public key to confirm the signature’s validity, ensuring the transaction is authentic and untampered.

Note that the `Witness` field is not included in computing the hash `TxId`. This is known as [SegWit](#segwit).
```go
type Witness struct {
	sig []byte
	pub []byte
}

func (witness *Witness) GetAddress() Address {
	return sha256.Sum256(witness.pub)
}
```
Bitcoin and its derivatives adopts a UTXO (unspent transaction output) model to prevent replay attacks (sending the same transaction twice). Each `TxIn` in a transaction refers to an unspent `TxOut` from a previous transaction. When included, the referred `TxOut` is completely spent. This way if two transactions refer to the same `TxOut`, the network will reject one of them to achieve consensus.
```go
type TxOut struct {
	Address Address
	Amount  uint64
}

type TxIn struct {
	TxId   TxId
	OutIdx uint64
}

type TxData struct {
	TxIns     []TxIn
	TxOuts    []TxOut
	Timestamp int64
}

func (txData *TxData) Hash() util.Hash {
	return util.NewHash(txData)
}
```
We can abstract creating and signing transactions with `Wallet`, a wrapper for the sender's private key.
```go
type Wallet ecdsa.PrivateKey

func (wallet *Wallet) GetPub() []byte {
	pub := wallet.PublicKey
	return elliptic.MarshalCompressed(pub.Curve, pub.X, pub.Y)
}

func (wallet *Wallet) GetAddress() Address {
	return sha256.Sum256(wallet.GetPub())
}
```

To create a transaction to another address, the sender searches for unspent `TxOuts` that his address owns.
```go
func (wallet *Wallet) sourceTxIns(utxoDb *UtxoDb, txData *TxData, amount uint64) (uint64, error) {
	address := wallet.GetAddress()
	for txIn := range utxoDb.uTxIns[address] {
		txData.TxIns = append(txData.TxIns, txIn)
		txOut, ok := utxoDb.mapTxInTxOut[txIn]
		if !ok {
			return 0, fmt.Errorf("txIn %v invalid", txIn)
		}
		if amount > txOut.Amount {
			amount -= txOut.Amount
		} else {
			return txOut.Amount - amount, nil
		}
	}
	if amount != 0 {
		return 0, fmt.Errorf("%s not enough funds", address)
	}
	return 0, nil
}
```

Since the `txOuts` are completely spent, there may be leftover balance. Some of these pay transaction fees; the remaining `change` is sent back to the sender.
```go
func (wallet *Wallet) MakeRegularTransaction(utxoDb *UtxoDb, recvAddress Address, amount uint64, transactionFee uint64) (*RegularTransaction, error) {
	if amount == 0 {
		return nil, fmt.Errorf("nothing to send")
	}
	txData := TxData{
		TxOuts:    []TxOut{{Address: recvAddress, Amount: amount}},
		Timestamp: time.Now().UnixMilli()}
	if change, err := wallet.sourceTxIns(utxoDb, &txData, amount+transactionFee); err != nil {
		return nil, err
	} else {
		if change != 0 {
			txOut := TxOut{Address: wallet.GetAddress(), Amount: change}
			txData.TxOuts = append(txData.TxOuts, txOut)
		}
	}
	txId := txData.Hash()
	txn := RegularTransaction{
		TransactionFee: transactionFee,
		TxId:           txId,
		TxData:         txData,
		Witness:        wallet.MakeWitness(txId)}
	return &txn, nil
}
```
These security measures prevent forgery but do not stop double-spending. Double-spending occurs when an attacker sends the same funds in two separate transactions—for example, paying a recipient in one transaction, then quickly spending the same balance in another before the first is confirmed. If successful, the attacker could invalidate the original payment while keeping the received goods or service.

To mitigate double spending, the transactions are committed to a blockchain. The receiver only delivers after the transaction is "buried" under several blocks (confirmations). This force te attacker to pay significant computational cost to invalidate the original transaction.
```go
type Block = blockchain.Block[BlockTransactions]
type Chain = blockchain.Chain[BlockTransactions]
```

### Rewards
Appending to the blockchain is costly. On top of this, there is a size limit to every block. Otherwise the resulting network latency gives unfair advantage to the most recent miner. Hence the miner receives a mining reward (`DEFAULT_COINBASE_AMOUNT`), which has the added benefit of increasing the "money" supply. On top of this the miner may charge fees for every transaction he commits to the blockchain. The two rewards are combined into a special transaction called `CoinbaseTransaction`.

Check that its address is also tamper-resistant, as changing it will change the `TxId` and eventually the `innerHash` of the `BlockTransactions`, forcing the attacker to find another `Nonce`.
```go
func transactionFees(txns []RegularTransaction) uint64 {
	var fees uint64
	for _, txn := range txns {
		fees += txn.TransactionFee
	}
	return fees
}

func NewBlockTransactions(txns []RegularTransaction, address Address) BlockTransactions {
	fees := transactionFees(txns)
	return BlockTransactions{CTxn: NewCoinbaseTransaction(address, DEFAULT_COINBASE_AMOUNT+fees), RTxns: txns}
}

func NewCoinbaseTransaction(address Address, amount uint64) CoinbaseTransaction {
	txData := TxData{
		TxOuts:    []TxOut{{address, amount}},
		Timestamp: time.Now().UnixMilli(),
	}
	return CoinbaseTransaction{
		TxId:   txData.Hash(),
		TxData: txData,
	}
}
```

### Transaction relaying
Transaction relaying allow those with limiting computing power to carry out transactions by paying fees. The user first broadcasts the transaction to nodes in the network. Nodes will keep these unconfirmed transactions in memory (known as `mempool`). Whenever a node is to mine a block, it includes in the block a set of non-conflicting transactions that maximize total transaction fees.
```go
func (node *Node) prepareNextUnmintedBlock() c.Block {
	node.mu.Lock()
	defer node.mu.Unlock()

	txns := node.protected.utxoDb.FilterRegularTransactions(node.protected.mempool)
	address := node.wallet.GetAddress()
	bt := c.NewBlockTransactions(txns, address)
	return node.protected.chain.NextUnmintedBlock(bt)
}
```
Check out the package `currency` for the full [code](currency).

## Appendix

### Simulation

The project includes a simulation program to help visualize how a cryptocurrency works. The network is a 2D mesh. All nodes are connected within the network, but messages can take up to two hops to reach another node. The channels `rBlock`, `rTxn`, `ssBlock` and `ssTxn` are for nodes to communicate with another through broadcasts. Messages are eventually delivered, in FIFO order.

Within a node, three goroutines `Relay()`, `Mine()`, and `Sim()` have concurrent access to `protected`, guarded with the Mutex `mu`. This simulates a node receiving and handling blocks and unconfirmed transactions from other nodes, while it mines blocks and processes new transactions from end users.
```go
type Node struct {
	mu        sync.Mutex
	protected struct {
		chain   c.Chain
		utxoDb  c.UtxoDb
		mempool []c.RegularTransaction // Assume validated
	}
	txIds   map[c.TxId]struct{}   // Exclusive to handleTransaction
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
```
See the full code [here](examples/currency/main.go).

### SegWit

Bitcoin supports "smart contracts". For example, user A can create a `TxOut` with specific spending conditions—such as requiring both user B and C to approve its use. To unlock this `TxOut`, the `TxIn` in the `TxData` section must include a valid signature known as `scriptSig`, proving that B and C have authorized the spend.

However, signatures are long and having multiple `scriptSig` wastes network bandwidth. More importantly, digital signatures are generally not unique. For example, the complement of an ECDSA signature is also a valid signature. An attacker can change the `TxId` of a legacy transaction easily by modifying one of the `scriptSig`. This is known as [transaction malleability](https://en.wikipedia.org/wiki/Transaction_malleability_problem).

Suppose Alice sends Eve money using transaction *T*. When Eve receives the yet-to-be-confirmed *T*, she broadcasts *T'* instead with a different but valid `scriptSig` and a different `TxId`. This *T'* is also valid and "signed" by Alice. If *T'* is confirmed (buried within the blockchain), Alice's transaction *T* will fail. A careless Alice will retry and send Eve the same amount with another transaction *S*, getting cheated by Eve. This is different from double spending.

This is why in a SegWit transaction, the `Witness` is purposefully omitted from computing `TxId`.
```go
type RegularTransaction struct {
	TransactionFee uint64
	TxId           TxId
	TxData         TxData
	Witness        Witness
}
```
