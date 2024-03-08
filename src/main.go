package main

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/core"
	"SilentPaymentAppBackend/src/db/mongodb"
	"SilentPaymentAppBackend/src/server"
	"fmt"
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
	fileDebug, err := os.OpenFile(fmt.Sprintf("./logs/logs-debug-%s.txt", time.Now()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	multi := io.MultiWriter(file, os.Stdout)
	//multiDebug := io.MultiWriter(fileDebug, os.Stdout)

	common.DebugLogger = log.New(fileDebug, "[DEBUG] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.InfoLogger = log.New(multi, "[INFO] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.WarningLogger = log.New(multi, "[WARNING] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.ErrorLogger = log.New(multi, "[ERROR] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)

	// load env vars
	catchUpRaw := os.Getenv("SYNC_CATCH_UP")
	if catchUpRaw != "" {
		catchUpRawConv, err := strconv.ParseUint(catchUpRaw, 10, 32)
		common.CatchUp = uint32(catchUpRawConv)
		if err != nil {
			common.ErrorLogger.Println(err)
		}
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

// todo redo the storage system,
//  after syncing ~5_500 blocks the estimated storage at 100_000 blocks for tweaks alone,
//  will be somewhere around 40Gb
//  Additionally performance is getting worse
// todo investigate whether rpc parallel calls can speed up syncing
//  caution: currently the flow is synchronous and hence there is less complexity making parallel calls will change that
// todo include redundancy for when rpc calls are failing (probably due to networking issues in testing home environment)
// todo review all duplicate key error exemptions and raise to error/warn from debug
// todo explore how many indices are too many,
//  in order to speed up queries I'm adding several indices
//  brings us back to potentially switching DB stack
// todo make a move to postgres instead of MongoDB
//  after revamp we do not need any nested structs
//  hence we can show everything in simple tables in postgres
// todo restructure tweak data storage to allow transaction cut-through
// todo remove unnecessary panics
// todo cut down on debug logging or find a more suitable solution for large logs
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	common.InfoLogger.Println("Program Started")
	// make sure everything is ready before we receive data
	mongodb.CreateIndices()

	// moved into go routine such that the interrupt signal will apply properly
	go func() {
		core.SyncChain()
		core.CheckForNewBlockRoutine()
	}()

	go server.RunServer(&server.ApiHandler{})

	for true {
		select {
		case <-interrupt:
			return
		}
	}
}
