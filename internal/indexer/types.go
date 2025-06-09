package indexer

type BlockData interface {
	GetBlockHash() [32]byte
	GetBlockHashSlice() []byte
	GetBlockHeight() uint32
	GetTransactions() []Transaction
}

type Transaction interface {
	GetTxId() [32]byte
	GetTxIdSlice() []byte
	GetTxIns() []TxIn
	GetTxOuts() []TxOut
}

type TxIn interface {
	GetTxId() [32]byte
	GetTxIdSlice() []byte
	GetVout() uint32
	// GetValue() uint64
	GetPrevoutPkScript() []byte // Previous output script
	GetPkScript() []byte
	GetWitness() [][]byte
	GetScriptSig() []byte
	// used to find coinbase txins, can be used to deactive other types txins as well
	Valid() bool
}

type TxOut interface {
	GetTxId() [32]byte
	GetTxIdSlice() []byte
	GetVout() uint32
	GetValue() uint64
	GetPkScript() []byte // Output script
}
