package main

import (
	"SilentPaymentAppBackend/src/db/mongodb"
	"SilentPaymentAppBackend/src/p2p"
	"SilentPaymentAppBackend/src/server"
	"SilentPaymentAppBackend/src/tweak"
	"encoding/hex"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"time"
)

func main() {
	//interrupt := make(chan os.Signal, 1)
	//signal.Notify(interrupt, os.Interrupt)

	// make sure everything is ready before we receive data
	mongodb.CreateIndices()

	messageOutChan := make(chan wire.Message, 100)
	foundTaprootTxChan := make(chan chainhash.Hash, 300)
	bestHeightChan := make(chan uint32)

	doneChan := make(chan struct{})
	fullySyncedChan := make(chan struct{})

	ph := p2p.PeerHandler{
		DoneChan:           doneChan,
		FoundTaprootTXChan: foundTaprootTxChan,
		MessageOutChan:     messageOutChan,
		SyncChan:           make(chan *wire.MsgHeaders),
		BestHeightChan:     bestHeightChan,
		FullySyncedChan:    fullySyncedChan,
	}
	go p2p.StartPeerRoutine(&ph, messageOutChan, doneChan)

	go tweak.StartFetchRoutine(foundTaprootTxChan, &ph)
	api := server.ApiHandler{
		BestHeightChan: bestHeightChan,
	}
	go api.HandleBestHeightUpdate()
	go server.RunServer(&api)
	//<-time.After(1 * time.Second)

	// wait for initial sync to be concluded
	<-fullySyncedChan

	for i, header := range ph.Headers {
		if i < 162460 {
			continue
		}
		bytes, err := hex.DecodeString(header.BlockHash.String())
		if err != nil {
			panic(err)
		}

		messageOutChan <- p2p.MakeBlockRequest(bytes, wire.InvTypeBlock)
	}

	//bytes, err := hex.DecodeString("0000009d659814e85a25bab650bb30a121d7ab091e7eeed99f7e884bbc80a5ae")
	//if err != nil {
	//	panic(err)
	//}
	//messageOutChan <- p2p.MakeBlockRequest(bytes, wire.InvTypeBlock)

	<-time.After(24 * time.Hour)
}
