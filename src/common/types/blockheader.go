package types

// BlockHeader struct to hold relevant BlockHeader data
// todo change naming to be consistent?
type BlockHeader struct {
	Hash          string `bson:"hash"`
	PrevBlockHash string `bson:"previousblockhash"`
	Timestamp     uint64 `bson:"timestamp"`
	Height        uint32 `bson:"height"`
}

func (v *BlockHeader) GetKey() []byte {
	return nil
}

func (v *BlockHeader) Serialise() []byte {
	return nil
}

func (v *BlockHeader) DeSerialise([]byte) error {
	return nil
}
