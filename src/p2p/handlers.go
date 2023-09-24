package p2p

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"time"
)
import (
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
)

type PeerHandler struct {
	DoneChan           chan struct{}
	FoundTaprootTXChan chan chainhash.Hash
	MessageOutChan     chan wire.Message
	BestHeightChan     chan uint32
	SyncChan           chan *wire.MsgHeaders
	HeaderChain        []*chainhash.Hash
	FullySyncedChan    chan struct{}
}

func (h *PeerHandler) onPing(p *peer.Peer, msg *wire.MsgPing) {
	fmt.Println("received ping")
	pongMsg := wire.NewMsgPong(msg.Nonce)
	p.QueueMessage(pongMsg, h.DoneChan)
}

func (h *PeerHandler) onPong(p *peer.Peer, msg *wire.MsgPong) {
	fmt.Println("received pong")
}

func (h *PeerHandler) onVersion(p *peer.Peer, msg *wire.MsgVersion) *wire.MsgReject {
	fmt.Printf("version: %+v\n", msg)
	h.BestHeightChan <- uint32(msg.LastBlock)
	return nil
}

func (h *PeerHandler) onVerack(p *peer.Peer, msg *wire.MsgVerAck) {
	fmt.Printf("verarck: %+v\n", msg)
	go h.InitialHeaderSync(h.FullySyncedChan)
}

func (h *PeerHandler) onInv(p *peer.Peer, msg *wire.MsgInv) {
	for _, invVec := range msg.InvList {
		if invVec.Type == wire.InvTypeBlock || invVec.Type == wire.InvTypeWitnessBlock {
			fmt.Printf("New Block %+v\n", invVec)
			// has to be converted due to a weird internal representation of a chainhash.hash
			bytesHash, err := hex.DecodeString(invVec.Hash.String())
			if err != nil {
				panic(err)
			}
			h.MessageOutChan <- MakeBlockRequest(bytesHash, wire.InvTypeBlock)
		}
	}
}

func (h *PeerHandler) onCFilter(p *peer.Peer, msg *wire.MsgCFilter) {

	mongodb.SaveFilter(&common.Filter{
		FilterType:  msg.FilterType,
		BlockHeight: uint32(common.IndexOfHash(&msg.BlockHash, h.HeaderChain)),
		Data:        msg.Data,
		BlockHeader: msg.BlockHash.String(),
	})
}

func (h *PeerHandler) onTx(p *peer.Peer, msg *wire.MsgTx) {
	fmt.Printf("%+v\n", msg)
	for _, txIn := range msg.TxIn {
		fmt.Printf("%+v\n", txIn)
	}
	for _, txOut := range msg.TxOut {
		fmt.Printf("%+v\n", txOut)
	}
}

func (h *PeerHandler) onHeaders(p *peer.Peer, msg *wire.MsgHeaders) {
	//fmt.Println(msg)
	h.SyncChan <- msg
}

func (h *PeerHandler) onBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	var perTransactionFlag bool
	h.AppendBlockerHeader(&msg.Header)
	<-time.After(5 * time.Second)

	// we are checking for the prev block => +2; we could check the current block but then it has to be 100% in the chain already
	thisBlocksHeight := common.IndexOfHash(&msg.Header.PrevBlock, h.HeaderChain) + 1
	fmt.Println(msg.Header.PrevBlock)
	fmt.Println(h.HeaderChain[len(h.HeaderChain)-5:])

	// -1 + 1 = 0
	if thisBlocksHeight == 0 {
		fmt.Println("[ERROR]", "got a block ahead of the chain, please re-sync, the headers")
		return
	}
	for _, tx := range msg.Transactions {
		perTransactionFlag = false
		for index, txOut := range tx.TxOut {
			if bytes.Equal(txOut.PkScript[:2], []byte{81, 32}) {
				if !perTransactionFlag {
					h.FoundTaprootTXChan <- tx.TxHash()
				}
				fmt.Println(tx.TxHash().String())
				mongodb.SaveLightUTXO(&common.LightUTXO{
					Txid:         tx.TxHash().String(),
					Vout:         uint32(index),
					Value:        uint64(txOut.Value),
					Scriptpubkey: hex.EncodeToString(txOut.PkScript),
					BlockHeight:  uint32(thisBlocksHeight),
				})
				perTransactionFlag = true
			}
		}
	}
	bytesHash, err := hex.DecodeString(msg.BlockHash().String())
	if err != nil {
		panic(err)
	}
	fmt.Println("CFilter height:", thisBlocksHeight)
	h.MessageOutChan <- MakeCFilterRequest(uint32(thisBlocksHeight), bytesHash)
}
