package common

var (
	MongoDBURI  = "mongodb://root:example@localhost:27017/"
	RpcEndpoint = "http://umbrel.local:8332/"
	RpcUser     = ""
	RpcPass     = ""
)

// control vars

var (
	CatchUp uint32 = 833_155 // random block where BIP352 was not merged yet. todo change to actual number
	// MinHeightToProcess No block below this number will be processed
	// todo is this actually needed
	//MinHeightToProcess uint32 = 833_000

	// SyncHeadersMaxPerCall how many headers will maximally be requested in one batched RPC call
	SyncHeadersMaxPerCall uint32 = 20000
)

var GenesisBlock = BlockHeader{
	Hash:          "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
	PrevBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
	Timestamp:     1231006505,
	Height:        0,
}

//const NumsH = "50929b74c1a04954b78b4b6035e97a5e078a5a0f28ec96d547bfee9ace803ac0"

var NumsH = []byte{80, 146, 155, 116, 193, 160, 73, 84, 183, 139, 75, 96, 53, 233, 122, 94, 7, 138, 90, 15, 40, 236, 150, 213, 71, 191, 238, 154, 206, 128, 58, 192}
