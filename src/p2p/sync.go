package p2p

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// todo add database component to this process, especially for production

func (h *PeerHandler) InitialHeaderSync(fullySynced chan struct{}) {
	allHeaders := mongodb.RetrieveAllHeaders()

	if len(allHeaders) == 0 {
		// todo change to appropriate network
		bytesHash, err := hex.DecodeString(chaincfg.SigNetParams.GenesisHash.String())
		if err != nil {
			panic(err)
		}
		// todo change to appropriate network
		h.Headers = append(h.Headers, &common.Header{
			BlockHash: chaincfg.SigNetParams.GenesisHash,
			Timestamp: 0,
		})
		h.MessageOutChan <- GetNewHeaders(bytesHash)
	} else {
		h.Headers = append(h.Headers, allHeaders...)
		lastHeader := allHeaders[len(allHeaders)-1]
		fmt.Println("Starting with hash", lastHeader.BlockHash.String())
		bytesHash, err := hex.DecodeString(lastHeader.BlockHash.String())
		if err != nil {
			panic(err)
		}
		h.MessageOutChan <- GetNewHeaders(bytesHash)
	}

	for true {
		select {
		case newMessage := <-h.SyncChan:
			var foundHeaders []*common.Header
			for _, header := range newMessage.Headers {
				hash := header.BlockHash()
				h.AppendBlockerHeader(header)
				//fmt.Println("New Header appended", hash)
				foundHeaders = append(foundHeaders, &common.Header{
					BlockHash: &hash,
					PrevBlock: &header.PrevBlock,
					Timestamp: uint32(header.Timestamp.Unix()),
				})
			}
			go mongodb.SaveBulkHeaders(foundHeaders)
			if len(newMessage.Headers) < 2000 {
				// will always send 2000 headers unless there are not more, so we have reached the end
				fullySynced <- struct{}{}
				fmt.Println("Headers fully synced")
			} else {
				bytesHash, err := hex.DecodeString(h.Headers[len(h.Headers)-1].BlockHash.String())

				if err != nil {
					panic(err)
				}
				h.MessageOutChan <- GetNewHeaders(bytesHash)
			}
		}
	}
}

func (h *PeerHandler) AppendBlockerHeader(header *wire.BlockHeader) {
	blockHash := header.BlockHash()
	// only append a new block hash if the previous block was the last known header i.e. this is the next following block
	if h.GetBlockHeightByHeader(&header.PrevBlock) == int32(len(h.Headers)-1) {
		h.Headers = append(h.Headers, &common.Header{
			BlockHash: &blockHash,
			PrevBlock: &header.PrevBlock,
			Timestamp: uint32(header.Timestamp.Unix()),
		})

		h.BestHeightChan <- uint32(len(h.Headers) - 1)
	}

}

func (h *PeerHandler) GetBlockHeightByHeader(headerHash *chainhash.Hash) int32 {
	return common.IndexOfHashInHeaderList(headerHash, h.Headers)
}

func (h *PeerHandler) GetTimestampByHeader(headerHash *chainhash.Hash) uint32 {
	return h.Headers[common.IndexOfHashInHeaderList(headerHash, h.Headers)].Timestamp
}

// reindexHeaderChain makes sure that the headers pulled from the database are in the correct order
func reindexHeaderChain() {

}
