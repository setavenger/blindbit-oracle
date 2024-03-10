package types

// LightUTXO could
// todo could be changed to unify spent UTXO and Light UTXO,
//  unused fields could just be omitted from serialisation and de-serialisation
type LightUTXO struct {
	Txid         string `json:"txid" bson:"txid"`
	Vout         uint32 `json:"vout" bson:"vout"`
	Value        uint64 `json:"value" bson:"value"`
	ScriptPubKey string `json:"scriptpubkey" bson:"scriptpubkey"`
	BlockHeight  uint32 `json:"block_height" bson:"block_height"`
	BlockHash    string `json:"block_hash" bson:"block_hash"`
	Timestamp    uint64 `json:"timestamp" bson:"timestamp"`
	TxidVout     string `json:"tx_id_vout" bson:"tx_id_vout"`
}

// SpentUTXO
// todo remove
// Deprecated: won't be stored and can hence be modified or replaced by a different struct type
type SpentUTXO struct {
	SpentIn     string `json:"spent_in" bson:"spentin"`
	Txid        string `json:"txid" bson:"txid"`
	Vout        uint32 `json:"vout" bson:"vout"`
	Value       uint64 `json:"value" bson:"value"`
	BlockHeight uint32 `json:"block_height" bson:"block_height"`
	BlockHash   string `json:"block_hash" bson:"block_hash"`
	Timestamp   uint64 `json:"timestamp" bson:"timestamp"`
}

func (v *LightUTXO) GetKey() []byte {
	return nil
}

func (v *LightUTXO) Serialise() []byte {
	return nil
}

func (v *LightUTXO) DeSerialise([]byte) error {
	return nil
}
