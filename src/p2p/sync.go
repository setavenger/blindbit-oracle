package p2p

import (
	"SilentPaymentAppBackend/src/common"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func (h *PeerHandler) InitialHeaderSync(fullySynced chan struct{}) {

	bytesHash, err := hex.DecodeString(chaincfg.RegressionNetParams.GenesisHash.String())
	if err != nil {
		panic(err)
	}
	h.HeaderChain = append(h.HeaderChain, chaincfg.RegressionNetParams.GenesisHash)
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

				bytesHash, err = hex.DecodeString(h.HeaderChain[len(h.HeaderChain)-1].String())
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
	if common.IndexOfHash(&header.PrevBlock, h.HeaderChain) == int32(len(h.HeaderChain)-1) {
		h.HeaderChain = append(h.HeaderChain, &blockHash)
		h.BestHeightChan <- uint32(len(h.HeaderChain) - 1)
	}

}

func (h *PeerHandler) GetBlockHeightByHeader(headerHash *chainhash.Hash) int32 {
	return common.IndexOfHash(headerHash, h.HeaderChain)
}
