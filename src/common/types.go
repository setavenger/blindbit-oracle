package common

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type LightUTXO struct {
	Txid         string `json:"txid"`
	Vout         uint32 `json:"vout"`
	Value        uint64 `json:"value"`
	Scriptpubkey string `json:"scriptpubkey"`
	BlockHeight  uint32 `json:"block_height"`
	BlockHeader  string `json:"block_header"`
	Timestamp    uint32 `json:"timestamp"`
}

type SpentUTXO struct {
	SpentIn     string `json:"spent_in"`
	Txid        string `json:"txid"`
	Vout        uint32 `json:"vout"`
	Value       uint64 `json:"value"`
	BlockHeight uint32 `json:"block_height"`
	BlockHeader string `json:"block_header"`
	Timestamp   uint32 `json:"timestamp"`
}

type Filter struct {
	FilterType  wire.FilterType `json:"filter_type"`
	BlockHeight uint32          `json:"block_height"`
	Data        []byte          `json:"data" bson:"data"`
	BlockHeader string          `json:"block_header"`
}

type TweakData struct {
	TxId        string   `json:"txid"`
	BlockHeight uint32   `json:"block_height"`
	Data        [32]byte `json:"data"` // todo change to 33bytes
}

type Transaction struct {
	//ID primitive.ObjectID `bson:"_id,omitempty"`
	//Vsize                int                `json:"vsize"`
	//FeePerVsize          float64            `json:"feePerVsize"`
	//EffectiveFeePerVsize float64            `json:"effectiveFeePerVsize"`
	Txid     string `json:"txid"`
	Version  int    `json:"version"`
	Locktime int    `json:"locktime"`
	//Size                 int                `json:"size"`
	//Weight               int                `json:"weight"`
	Fee    uint32            `json:"fee"`
	Vin    []Vin             `json:"vin"`
	Vout   []Vout            `json:"vout"`
	Status TransactionStatus `json:"status"`
}

type Vin struct {
	IsCoinbase bool    `json:"is_coinbase"`
	Prevout    Prevout `json:"prevout"`
	Scriptsig  string  `json:"scriptsig"`
	//ScriptsigAsm          string   `json:"scriptsig_asm"`
	//Sequence              uint32   `json:"sequence"`
	Txid                  string   `json:"txid"`
	Vout                  uint32   `json:"vout"`
	Witness               []string `json:"witness"`
	InnerRedeemscriptAsm  string   `json:"inner_redeemscript_asm"`
	InnerWitnessscriptAsm string   `json:"inner_witnessscript_asm"`
}

type Prevout struct {
	Value        uint64 `json:"value"`
	Scriptpubkey string `json:"scriptpubkey"`
	//ScriptpubkeyAddress string `json:"scriptpubkey_address"`
	//ScriptpubkeyAsm     string `json:"scriptpubkey_asm"`
	ScriptpubkeyType string `json:"scriptpubkey_type"`
}

type Vout struct {
	Value        uint64 `json:"value"`
	Scriptpubkey string `json:"scriptpubkey"`
	//ScriptpubkeyAddress string `json:"scriptpubkey_address"`
	//ScriptpubkeyAsm     string `json:"scriptpubkey_asm"`
	ScriptpubkeyType string `json:"scriptpubkey_type"`
}

type TransactionStatus struct {
	Confirmed   bool   `json:"confirmed"`
	BlockHeight uint32 `json:"block_height"`
	BlockHash   string `json:"block_hash"`
	//BlockTime   int    `json:"block_time"`
}

type Header struct {
	BlockHash *chainhash.Hash `bson:"block_hash"`
	PrevBlock *chainhash.Hash `bson:"prev_block"`
	Timestamp uint32          `bson:"timestamp"`
}
