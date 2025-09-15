package dbpebble

const (
	SizeHash   = 32
	SizeTxid   = 32
	SizeHeight = 4
	SizePos    = 4
	SizeVout   = 4
	SizeTweak  = 33
	SizeAmt    = 8
	SizePubKey = 32 // x-only pubkey
)

// Prefix Keys "K"
const (
	KBlockTx  = 0x01
	KTx       = 0x02
	KOut      = 0x03
	KSpend    = 0x04
	KCIHeight = 0x05
	KCIBlock  = 0x06
	KTxOccur  = 0x07

	// Statics
	KTweaksStatic = 0x08
	KUTXOsStatic  = 0x09

	/* Filters */

	// Inlcudes all taproot outputs for block
	KTaprootPubkeyFilter = 0x0A

	// Inlcudes all taproot unspent outputs for block
	KTaprootUnspentFilter = 0x0B

	// Inlcudes all taproot spent outputs for block
	KTaprootSpentFilter = 0x0C

	// Compute Index
	KComputeIndex = 0x0D
)
