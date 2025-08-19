// database defines the interfaces for for handling db operations
package database

import "github.com/btcsuite/btcd/chaincfg/chainhash"

type DB interface {
	ApplyBlock(*DBBlock) error
}

type DBBlock struct {
	Height uint32
	Hash   *chainhash.Hash
	Txs    []*Tx
}

type Out struct {
	Txid   []byte
	Vout   uint32
	Amount uint64
	Pubkey []byte // 32B x-only
}

type In struct {
	SpendTxid []byte // not needed for core queries; keep if you add a spender→prevout index
	Idx       uint32
	PrevTxid  []byte
	PrevVout  uint32
	Pubkey    []byte // optional 32B (taproot key path spend x-only)
}

type Tx struct {
	Txid  []byte
	Tweak *[33]byte // 33B or nil
	Outs  []*Out
	Ins   []*In
}
