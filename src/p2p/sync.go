package p2p

import (
	"SilentPaymentAppBackend/src/common"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// todo add database component to this process, especially for production

func (h *PeerHandler) InitialHeaderSync(fullySynced chan struct{}) {

	bytesHash, err := hex.DecodeString(chaincfg.RegressionNetParams.GenesisHash.String())
	if err != nil {
		panic(err)
	}
	h.Headers = append(h.Headers, &common.Header{
		Hash:      chaincfg.RegressionNetParams.GenesisHash,
		Timestamp: 0,
	})
	h.MessageOutChan <- GetNewHeaders(bytesHash)
	for true {
		select {
		case newMessage := <-h.SyncChan:
			for _, header := range newMessage.Headers {
				hash := header.BlockHash()
				h.AppendBlockerHeader(header)
				fmt.Println("New Header appended", hash)

			}
			if len(newMessage.Headers) < 2000 {
				// will always send 2000 headers unless there are not more, so we have reached the end
				fullySynced <- struct{}{}
			} else {
				bytesHash, err = hex.DecodeString(h.Headers[len(h.Headers)-1].Hash.String())

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
			Hash:      &blockHash,
			Timestamp: uint32(header.Timestamp.Unix()),
		})

		h.BestHeightChan <- uint32(len(h.Headers) - 1)
	}

}

func (h *PeerHandler) GetBlockHeightByHeader(headerHash *chainhash.Hash) int32 {
	return common.IndexOfHashInHeaderList(headerHash, h.Headers)
}
