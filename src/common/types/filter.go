package types

type Filter struct {
	FilterType  uint8  `json:"filter_type" bson:"filter_type"`
	BlockHeight uint32 `json:"block_height" bson:"block_height"`
	Data        []byte `json:"data" bson:"data"`
	BlockHash   string `json:"block_hash" bson:"block_hash"`
}

func (v *Filter) GetKey() ([]byte, error) {
	return nil, nil
}

func (v *Filter) Serialise() ([]byte, error) {
	return nil, nil
}

func (v *Filter) DeSerialise([]byte) error {
	return nil
}
