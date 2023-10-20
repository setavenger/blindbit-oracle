package p2p

import (
	"SilentPaymentAppBackend/src/common"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func MakeBlockRequest(blockHeader []byte, invType wire.InvType) *wire.MsgGetData {

	bytes := blockHeader[:32]

	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}
	hash := chainhash.Hash{}
	err := hash.SetBytes(bytes)

	if err != nil {
		panic(err)
	}
	return &wire.MsgGetData{
		InvList: []*wire.InvVect{
			wire.NewInvVect(invType, &hash),
		},
	}
}

func MakeCFilterRequest(startHeight uint32, blockHeader []byte) *wire.MsgGetCFilters {

	bytes := blockHeader[:32]

	hash := chainhash.Hash{}
	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}
	err := hash.SetBytes(bytes)
	if err != nil {
		panic(err)
	}
	return &wire.MsgGetCFilters{
		FilterType:  wire.GCSFilterRegular,
		StartHeight: startHeight,
		StopHash:    hash,
	}
}

func GetNewHeaders(lastBlockHeader []byte) *wire.MsgGetHeaders {
	bytes := lastBlockHeader[:32]

	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}
	hash := chainhash.Hash{}
	err := hash.SetBytes(bytes)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil
	}
	return &wire.MsgGetHeaders{
		ProtocolVersion:    70016,
		BlockLocatorHashes: []*chainhash.Hash{&hash},
		HashStop:           [32]byte{},
	}
}
