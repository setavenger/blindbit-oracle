package types

type Tweak struct {
	BlockHash   string   `json:"block_hash" bson:"block_hash"`
	BlockHeight uint32   `json:"block_height" bson:"block_height"`
	Txid        string   `json:"txid" bson:"txid"`
	Data        [33]byte `json:"data"`
}

func (v *Tweak) GetKey() []byte {
	return nil
}

func (v *Tweak) Serialise() []byte {
	return nil
}

func (v *Tweak) DeSerialise([]byte) error {
	return nil
}
