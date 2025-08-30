// Package database defines the interfaces for for handling db operations
package database

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type DB interface {
	GetChainTip() ([]byte, uint32, error)
	GetBlockHashByHeight(height uint32) ([]byte, error)
	ApplyBlock(*DBBlock) error
	FlushBatch() error
	TweaksForBlockAll([]byte) ([]*TweakRow, error)
	TweaksForBlockCutThrough([]byte, uint32) ([]TweakRow, error)
	FetchOutputsAll(blockhash []byte, tipheight uint32) ([]*Output, error)
	BatchSize() int
}

type TweakRow struct {
	Txid  [32]byte
	Tweak [33]byte
}

type DBBlock struct {
	Height uint32
	Hash   *chainhash.Hash
	Txs    []*Tx
}

type Output struct {
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
	Outs  []*Output
	Ins   []*In
}
