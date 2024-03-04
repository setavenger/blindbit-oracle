package main

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/core"
	"SilentPaymentAppBackend/src/db/mongodb"
	"SilentPaymentAppBackend/src/p2p"
	"SilentPaymentAppBackend/src/server"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
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

	multi := io.MultiWriter(file, os.Stdout)

	common.DebugLogger = log.New(multi, "[DEBUG] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.InfoLogger = log.New(multi, "[INFO] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.WarningLogger = log.New(multi, "[WARNING] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.ErrorLogger = log.New(multi, "[ERROR] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)

	// load env vars
	catchUpRaw := os.Getenv("SYNC_CATCH_UP")
	common.CatchUp, err = strconv.ParseUint(catchUpRaw, 10, 64)
	if err != nil {
		common.ErrorLogger.Println(err)
	}

	mongoDBConnection := os.Getenv("MONGO_DB_CONNECTION")
	if mongoDBConnection != "" {
		common.MongoDBURI = mongoDBConnection
	}

	rpcUser := os.Getenv("RPC_USER")
	if rpcUser != "" {
		common.RpcUser = rpcUser
	} else {
		panic("rpc user not set")
	}

	rpcPass := os.Getenv("RPC_PASS")
	if rpcPass != "" {
		common.RpcPass = rpcPass
	} else {
		panic("rpc pass not set")
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
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

	go core.StartFetchRoutine(foundTaprootTxChan, &ph)
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
	// first transactions around 163000
	if common.CatchUp > 0 {
		for i, header := range ph.Headers {
			if uint64(i) < common.CatchUp {
				continue
			}
			bytes, err := hex.DecodeString(header.BlockHash.String())
			if err != nil {
				panic(err)
			}

			messageOutChan <- p2p.MakeBlockRequest(bytes, wire.InvTypeBlock)
			if i != len(ph.Headers)-1 {
				<-time.After(1000 * time.Millisecond)
			}
		}
	}

	//bytes, err := hex.DecodeString("000000f88b466c41306080c0778ed1eea80cb185b4b80a803840a91c79df52c7")
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
