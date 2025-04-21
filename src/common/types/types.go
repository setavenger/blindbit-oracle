package types

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
	Height       uint32       `json:"height"`
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

// RPC Types

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type ErrorRPC string

//func (e ErrorRPC) Error() string {
//	return fmt.Sprintf("%s", e)
//}

// RPCResponseBlock represents a JSON RPC response for GetBlock
type RPCResponseBlock struct {
	ID    string   `json:"id"`
	Block Block    `json:"result,omitempty"`
	Error ErrorRPC `json:"error,omitempty"`
}

// RPCResponseHeader represents a JSON RPC response for getblockheader
type RPCResponseHeader struct {
	ID     string         `json:"id"`
	Result BlockHeaderRPC `json:"result,omitempty"`
	Error  ErrorRPC       `json:"error,omitempty"`
}

// BlockHeaderRPC represents the structure of a block header in the response
type BlockHeaderRPC struct {
	Hash              string `json:"hash"`
	Height            uint32 `json:"height"`
	Timestamp         uint64 `json:"time"`
	PreviousBlockHash string `json:"previousblockhash"`
	NextBlockHash     string `json:"nextblockhash"`
}

type RPCResponseBlockchainInfo struct {
	ID     string         `json:"id"`
	Result BlockchainInfo `json:"result,omitempty"`
	Error  interface{}    `json:"error,omitempty"`
}

// BlockchainInfo represents the structure of the blockchain information
type BlockchainInfo struct {
	Chain         string `json:"chain"`
	Blocks        uint32 `json:"blocks"` // The current number of blocks processed in the server
	Headers       uint32 `json:"headers"`
	BestBlockHash string `json:"bestblockhash"`
}

type RPCResponseSendRawTransaction struct {
	ID     string      `json:"id"`
	Result string      `json:"result,omitempty"`
	Error  interface{} `json:"error,omitempty"`
}
