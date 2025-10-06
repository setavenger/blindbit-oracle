// Package database defines the interfaces for for handling db operations
package database

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/setavenger/blindbit-lib/proto/pb"
)

type DB interface {
	GetChainTip() ([]byte, uint32, error)
	GetBlockHashByHeight(height uint32) ([]byte, error)
	ApplyBlock(*DBBlock) error
	FlushBatch(sync bool) error
	TweaksForBlockAll([]byte) ([]*TweakRow, error)
	TweaksForBlockCutThrough([]byte, uint32) ([]TweakRow, error)
	FetchOutputsAll(blockhash []byte, tipheight uint32) ([]*Output, error)
	FetchTweaksStatic(blockhash []byte) ([][]byte, error)
	FetchTweaksStaticProto(blockhash []byte) ([][]byte, error) // todo: i guess redundant
	FetchOutputsStatic(blockhash []byte) ([]*Output, error)
	FetchOutputsStaticProto(blockhash []byte) ([]pb.UTXO, error)
	FetchTaprootUnspentFilter(blockhash []byte) ([]byte, error)
	FetchSpentOutputs(blockhash []byte) ([]byte, error)
	FetchSpentOutpointsAccelerator(blockhash []byte) ([][36]byte, error)
	ChainIterator(asc bool) (<-chan []byte, error) // todo: add context
	FetchComputeIndex(height uint32) ([]*pb.ComputeIndexTxItem, error)
	BlockhashInDB(blockhash []byte) (bool, error)
	BatchSize() int
	ReindexBlock(blockhash []byte, force bool) error
	ReindexBlockWithOptions(blockhash []byte, forceAll bool, forceInexpensive bool) error
	BuildStaticIndexesForBlock(blockhash []byte) error
	BuildAcceleratorIndexesForBlock(blockhash []byte) error

	// Static index existence checks
	KeyExistsStaticTweaks(blockhash []byte) (bool, error)
	KeyExistsStaticOutputs(blockhash []byte) (bool, error)
	KeyExistsStaticTaprootUnspentFilter(blockhash []byte) (bool, error)
	KeyExistsSpentOutpointsAccelerator(blockhash []byte) (bool, error)
	KeyExistsComputeIndex(blockhash []byte) (bool, error)
}

type TweakRow struct {
	Txid  [32]byte
	Tweak [33]byte
}

type DBBlock struct {
	Height uint32
	Hash   *chainhash.Hash
	Txs    []*Tx
	// todo: add filters
}

type Output struct {
	Txid   []byte // todo: should probably be an array
	Vout   uint32
	Amount uint64
	Pubkey []byte // 32B x-only todo: should probably be an array
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
