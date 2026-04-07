package indexer

// // PrepareDBTx takes the inputs necessary to build the db types of a tx to efficiently insert a tx within a block
// // tweak is already byte slice (instead of normal 32 byte array) so nil is nil. Check has to be done prior.
// func PrepareDBTx(tx *Transaction, tweak []byte, vins []*Vin, outs []*wire.TxOut) *database.Tx {
// 	bTx := database.Tx{
// 		Txid:  tx.txid[:],
// 		Tweak: *tweak[:],
// 		Outs:  []database.Out{},
// 		Ins:   []database.In{},
// 	}
//
// 	return &bTx
// }
