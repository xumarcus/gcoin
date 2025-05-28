package currency

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"gcoin/blockchain"
	"gcoin/util"
)

const DEFAULT_COINBASE_AMOUNT = 50

func Unmarshal(pub []byte) ecdsa.PublicKey {
	Curve := elliptic.P256()
	X, Y := elliptic.UnmarshalCompressed(Curve, pub)
	return ecdsa.PublicKey{
		Curve: Curve,
		X:     X,
		Y:     Y,
	}
}

type Address = util.Hash
type TxId = util.Hash

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

type Witness struct {
	sig []byte
	pub []byte
}

func (witness *Witness) GetAddress() Address {
	return sha256.Sum256(witness.pub)
}

type Block = blockchain.Block[BlockTransactions]
type Chain = blockchain.Chain[BlockTransactions]
