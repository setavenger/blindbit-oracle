package main

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"SilentPaymentAppBackend/src/p2p"
	"SilentPaymentAppBackend/src/server"
	"SilentPaymentAppBackend/src/tweak"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
)

func init() {
	err := os.Mkdir("./logs", 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		fmt.Println(err.Error())
		log.Fatal(err)
	}

	file, err := os.OpenFile(fmt.Sprintf("./logs/logs-%s.txt", time.Now()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	common.DebugLogger = log.New(file, "[DEBUG] ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
	common.InfoLogger = log.New(file, "[INFO] ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
	common.WarningLogger = log.New(file, "[WARNING] ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
	common.ErrorLogger = log.New(file, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	common.InfoLogger.Println("Program Started")
	// make sure everything is ready before we receive data
	mongodb.CreateIndices()

	messageOutChan := make(chan wire.Message, 100)
	foundTaprootTxChan := make(chan chainhash.Hash, 300)
	bestHeightChan := make(chan uint32)

	doneChan := make(chan struct{})
	fullySyncedChan := make(chan struct{})
	peerRoutineEndedChan := make(chan struct{})

	ph := p2p.PeerHandler{
		DoneChan:           doneChan,
		FoundTaprootTXChan: foundTaprootTxChan,
		MessageOutChan:     messageOutChan,
		SyncChan:           make(chan *wire.MsgHeaders),
		BestHeightChan:     bestHeightChan,
		FullySyncedChan:    fullySyncedChan,
	}
	go p2p.StartPeerRoutine(&ph, messageOutChan, doneChan, peerRoutineEndedChan)

	go tweak.StartFetchRoutine(foundTaprootTxChan, &ph)
	api := server.ApiHandler{
		BestHeightChan: bestHeightChan,
	}
	go api.HandleBestHeightUpdate()
	go server.RunServer(&api)
	//<-time.After(1 * time.Second)

	// wait for initial sync to be concluded
	<-fullySyncedChan

	// 162458 start of the signet experiments
	// todo 162591 was error with p2pkh input, use for testing (b9d5c5dceed52098e0aa4529e55f5279b79bd510fce8429d6b0914e10215279f)
	//for i, header := range ph.Headers {
	//	if i < 162458 {
	//		continue
	//	}
	//	bytes, err := hex.DecodeString(header.BlockHash.String())
	//	if err != nil {
	//		panic(err)
	//	}
	//
	//	messageOutChan <- p2p.MakeBlockRequest(bytes, wire.InvTypeBlock)
	//	if i != len(ph.Headers)-1 {
	//		<-time.After(400 * time.Millisecond)
	//	}
	//}

	//bytes, err := hex.DecodeString("000000e76341456b13358d7efb851c33413c122ffaedabea7eef53324f8d7711")
	//if err != nil {
	//	panic(err)
	//}
	//messageOutChan <- p2p.MakeBlockRequest(bytes, wire.InvTypeBlock)

	for true {

		select {
		case <-peerRoutineEndedChan:
			common.InfoLogger.Println("Reconnecting to Peer")
			go p2p.StartPeerRoutine(&ph, messageOutChan, doneChan, peerRoutineEndedChan)
		case <-interrupt:
			return
		}
	}
}
