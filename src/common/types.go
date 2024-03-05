package common

import (
	"github.com/btcsuite/btcd/wire"
)

type LightUTXO struct {
	TxId         string `json:"txid"`
	Vout         uint32 `json:"vout"`
	Value        uint64 `json:"value"`
	ScriptPubKey string `json:"scriptpubkey"`
	BlockHeight  uint32 `json:"block_height"`
	BlockHash    string `json:"block_hash"`
	Timestamp    uint64 `json:"timestamp"`
}

type SpentUTXO struct {
	SpentIn     string `json:"spent_in"`
	Txid        string `json:"txid"`
	Vout        uint32 `json:"vout"`
	Value       uint64 `json:"value"`
	BlockHeight uint32 `json:"block_height"`
	BlockHash   string `json:"block_header"`
	Timestamp   uint64 `json:"timestamp"`
}

type Filter struct {
	FilterType  wire.FilterType `json:"filter_type"`
	BlockHeight uint32          `json:"block_height"`
	Data        []byte          `json:"data" bson:"data"`
	BlockHeader string          `json:"block_header"`
}

type TweakIndex struct {
	BlockHash   string     `json:"blockHash" bson:"block_hash"`
	BlockHeight uint32     `json:"block_height" bson:"block_height"`
	Data        [][33]byte `json:"data"`
}

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
	//Size     int    `json:"size"`
	//Vsize    int    `json:"vsize"`
	//Weight   int    `json:"weight"`
	//Locktime int    `json:"locktime"`
	Vin  []Vin  `json:"vin"`
	Vout []Vout `json:"vout"`
	//Hex      string `json:"hex"`  // not used to cut down on data
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
