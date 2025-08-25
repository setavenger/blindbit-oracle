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

const (
	KBlockTx  = 0x01
	KTx       = 0x02
	KOut      = 0x03
	KSpend    = 0x04
	KCIHeight = 0x05
	KCIBlock  = 0x06
	KTxOccur  = 0x07
)
