package main

import (
	"SilentPaymentAppBackend/src/db/mongodb"
	"SilentPaymentAppBackend/src/p2p"
	"SilentPaymentAppBackend/src/server"
	"SilentPaymentAppBackend/src/tweak"
	"encoding/hex"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"log"
	"os"
	"os/signal"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// make sure everything is ready before we receive data
	mongodb.CreateIndices()

	messageOutChan := make(chan wire.Message, 10)
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

	bytes, err := hex.DecodeString("33d717cce7160b51351f22b283b8ea2ab0e3f5ee41c06bde717dc499dbb58a91")
	if err != nil {
		panic(err)
	}
	//_ = bytes
	//
	<-fullySyncedChan
	messageOutChan <- p2p.MakeBlockRequest(bytes, wire.InvTypeBlock)
	//transactions := mongodb.RetrieveByBlockHeight(806758)
	//fmt.Println(transactions[0])
	for {
		select {

		case <-interrupt:
			log.Println("interrupt")
			return
		}
	}
}
