package common

// TaprootActivation
// todo might be inapplicable due to transactions that have taproot prevouts from before the activation
//  is relevant for the height-to-hash lookup in the db
const TaprootActivation uint32 = 709632

var (
	RpcEndpoint = "http://127.0.0.1:8332" // default local node
	RpcUser     = ""
	RpcPass     = ""
)

// control vars
var (
	CatchUp uint32 = 833_000 // random block where BIP352 was not active yet. todo change to actual number
	// MinHeightToProcess No block below this number will be processed
	// todo is this actually needed
	//MinHeightToProcess uint32 = 833_000

	// SyncHeadersMaxPerCall how many headers will maximally be requested in one batched RPC call
	SyncHeadersMaxPerCall        uint32 = 10_000
	MaxParallelRequests          uint8  = 12
	MaxParallelTweakComputations        = 12
)

// NumsH = 0x50929b74c1a04954b78b4b6035e97a5e078a5a0f28ec96d547bfee9ace803ac0
var NumsH = []byte{80, 146, 155, 116, 193, 160, 73, 84, 183, 139, 75, 96, 53, 233, 122, 94, 7, 138, 90, 15, 40, 236, 150, 213, 71, 191, 238, 154, 206, 128, 58, 192}
