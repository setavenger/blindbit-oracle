package common

const MinHeightToProcess = 833_000

var (
	CatchUp uint64 = 0 // random block where BIP352 was not merged yet. todo change to actual number

	MongoDBURI  = "mongodb://root:example@localhost:27017/"
	RpcEndpoint = "http://umbrel.local:8332/"
	RpcUser     = "umbrel"
	RpcPass     = ""
)

var GenesisBlock = BlockHeader{
	Hash:          "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
	PrevBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
	Timestamp:     1231006505,
	Height:        0,
}
