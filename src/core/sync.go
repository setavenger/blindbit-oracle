package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
)

func SyncChain() {
	common.InfoLogger.Println("starting sync")

	lastHeader, err := mongodb.RetrieveLastHeader()
	if err != nil {
		// fatal due to startup condition
		common.ErrorLogger.Fatalln(err)
		return
	}

	blockchainInfo, err := GetBlockchainInfo()
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return
	}
	common.DebugLogger.Printf("blockchain info: %+v", blockchainInfo)

	syncFromHeight := lastHeader.Height
	if syncFromHeight < common.CatchUp {
		syncFromHeight = common.CatchUp
	}

	// how many headers are supposed to be fetched at once
	step := common.SyncHeadersMaxPerCall
	for i := syncFromHeight; i < blockchainInfo.Blocks; {
		// Adjust for the last run when there are fewer headers left than the step; avoids index out of range
		if i+step > blockchainInfo.Blocks {
			step = blockchainInfo.Blocks - i + 1 // needs to be +1 because GetBlockHeadersBatch starts at start height and is hence technically zero indexed
		}

		var headers []BlockHeader
		headers, err = GetBlockHeadersBatch(i, step)
		if err != nil {
			common.ErrorLogger.Println(err)
			return
		}

		for _, header := range headers {
			CheckBlockHash(header.Hash)
		}

		// Increment 'i' by 'step' after processing the headers
		i += step
	}

}
