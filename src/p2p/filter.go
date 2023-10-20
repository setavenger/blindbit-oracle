package p2p

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/gcs"
	"log"
)

func BuildTaprootOnlyFilter(block *wire.MsgBlock, ph *PeerHandler) []byte {
	// we are checking for the prev block => +1; we could check the current block but then it has to be 100% in the chain already
	thisBlocksHeight := ph.GetBlockHeightByHeader(&block.Header.PrevBlock) + 1

	// -1 + 1 = 0
	if thisBlocksHeight == 0 {
		log.Println("[ERROR]", "got a block ahead of the chain, please re-sync, the headers")
		return nil
	}

	var taprootOutput [][]byte

	hash := block.Header.BlockHash()

	for _, tx := range block.Transactions {
		for _, txOut := range tx.TxOut {
			if bytes.Equal(txOut.PkScript[:2], []byte{81, 32}) {
				taprootOutput = append(taprootOutput, txOut.PkScript)
			}
		}
	}

	// todo might need to be reversed
	key := builder.DeriveKey(&hash)

	filter, err := gcs.BuildGCSFilter(builder.DefaultP, builder.DefaultM, key, taprootOutput)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil
	}

	nBytes, err := filter.NBytes()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil
	}
	return nBytes

}
