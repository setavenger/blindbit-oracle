package common

import (
	"github.com/btcsuite/btcd/wire"
)

type LightUTXO struct {
	Txid         string `json:"txid" bson:"txid"`
	Vout         uint32 `json:"vout" bson:"vout"`
	Value        uint64 `json:"value" bson:"value"`
	ScriptPubKey string `json:"scriptpubkey" bson:"scriptpubkey"`
	BlockHeight  uint32 `json:"block_height" bson:"block_height"`
	BlockHash    string `json:"block_hash" bson:"block_hash"`
	Timestamp    uint64 `json:"timestamp" bson:"timestamp"`
	TxidVout     string `json:"tx_id_vout" bson:"tx_id_vout"`
}

type SpentUTXO struct {
	SpentIn     string `json:"spent_in" bson:"spentin"`
	Txid        string `json:"txid" bson:"txid"`
	Vout        uint32 `json:"vout" bson:"vout"`
	Value       uint64 `json:"value" bson:"value"`
	BlockHeight uint32 `json:"block_height" bson:"block_height"`
	BlockHash   string `json:"block_hash" bson:"block_hash"`
	Timestamp   uint64 `json:"timestamp" bson:"timestamp"`
}

type Filter struct {
	FilterType  wire.FilterType `json:"filter_type" bson:"filter_type"`
	BlockHeight uint32          `json:"block_height" bson:"block_height"`
	Data        []byte          `json:"data" bson:"data"`
	BlockHash   string          `json:"block_hash" bson:"block_hash"`
}

type Tweak struct {
	BlockHash   string   `json:"block_hash" bson:"block_hash"`
	BlockHeight uint32   `json:"block_height" bson:"block_height"`
	Txid        string   `json:"txid" bson:"txid"`
	Data        [33]byte `json:"data"`
}

// BlockHeader struct to hold relevant BlockHeader data
// todo change naming to be consistent?
type BlockHeader struct {
	Hash          string `bson:"hash"`
	PrevBlockHash string `bson:"previousblockhash"`
	Timestamp     uint64 `bson:"timestamp"`
	Height        uint32 `bson:"height"`
}

// Block represents the structure of the block data in the RPC response
type Block struct {
	Hash              string        `json:"hash"`
	Height            uint32        `json:"height"`
	PreviousBlockHash string        `json:"previousblockhash"`
	Timestamp         uint64        `json:"time"`
	Txs               []Transaction `json:"tx"`
}

// Transaction represents the structure of a transaction in the block
type Transaction struct {
	Txid    string `json:"txid"`
	Hash    string `json:"hash"`
	Version int    `json:"version"`
	Vin     []Vin  `json:"vin"`
	Vout    []Vout `json:"vout"`
}

// Vin represents a transaction input
type Vin struct {
	Txid        string    `json:"txid"`
	Vout        uint32    `json:"vout"`
	ScriptSig   ScriptSig `json:"scriptSig"`
	Txinwitness []string  `json:"txinwitness,omitempty"`
	Sequence    uint32    `json:"sequence"`
	Prevout     *Prevout  `json:"prevout,omitempty"`
	Coinbase    string    `json:"coinbase"`
}

// Vout represents a transaction output
type Vout struct {
	Value        float64      `json:"value"`
	N            uint32       `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}

// Prevout represents the previous output for an input
type Prevout struct {
	Generated    bool         `json:"generated"`
	Height       int          `json:"height"`
	Value        float64      `json:"value"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}

// ScriptPubKey represents the script key
type ScriptPubKey struct {
	Asm     string `json:"asm"`
	Desc    string `json:"desc"`
	Hex     string `json:"hex"`
	Address string `json:"address,omitempty"`
	Type    string `json:"type"`
}

type ScriptSig struct {
	ASM string `json:"asm"`
	Hex string `json:"hex"`
}
