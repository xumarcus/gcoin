package currency

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"fmt"
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

func (txOut TxOut) String() string {
	return fmt.Sprintf("$%d->%s", txOut.Amount, txOut.Address)
}

type TxIn struct {
	TxId   TxId
	OutIdx uint64
}

func (txIn TxIn) String() string {
	return fmt.Sprintf("%s[%d]", txIn.TxId, txIn.OutIdx)
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
