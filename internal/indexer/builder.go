package indexer

type Builder struct {
	CurrentHeight int64
	// pulled Blocks end up here
	newBlockChan chan Block
}
