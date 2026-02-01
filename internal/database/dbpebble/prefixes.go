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
	/* Basic data */
	KBlockTx  = 0x01 // blockhash+position -> txid
	KTx       = 0x02 // txid -> tweak
	KOut      = 0x03 // txid+vout -> amount+pubkey
	KSpend    = 0x04 // prev_txid+prev_vout+blockhash -> spend_pubkey
	KCIHeight = 0x05 // height -> blockhash
	KCIBlock  = 0x06 // blockhash -> height
	KTxOccur  = 0x07 // txid+blockhash -> nil

	// Compute Index
	KComputeIndex = 0x0D // height+txid -> tweak+outputs_short

	// Spent Outputs Short (first 8 bytes of x-only pubkeys)
	KSpentOutputsShort = 0x0E // blockhash -> spent_outputs_short

	/* Accelerators */

	// Txid to Outpoints mapping (blockhash+txid -> concatenated outpoints)
	KTxidOutpoints = 0x0F // blockhash+txid -> outpoints
)
