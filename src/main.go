package main

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/core"
	"SilentPaymentAppBackend/src/db/dblevel"
	"SilentPaymentAppBackend/src/server"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof" // Import for side effects: registers pprof handlers with the default mux.
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

func init() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
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
	common.ErrorLogger = log.New(multi, "[ERROR] ", log.Ldate|log.Lmicroseconds|log.Llongfile|log.Lmsgprefix)

	// create DB path
	err = os.Mkdir("./data", 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		fmt.Println(err.Error())
		log.Fatal(err)
	}

	// load env vars
	catchUpRaw := os.Getenv("SYNC_CATCH_UP")
	if catchUpRaw != "" {
		catchUpRawConv, err := strconv.ParseUint(catchUpRaw, 10, 32)
		common.CatchUp = uint32(catchUpRawConv)
		if err != nil {
			common.ErrorLogger.Println(err)
		}
	}

	// open levelDB connections
	openLevelDBConnections()

	rpcEndpoint := os.Getenv("RPC_ENDPOINT")
	if rpcEndpoint != "" {
		common.RpcEndpoint = rpcEndpoint
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

// todo investigate whether we should change the compound keys to use the height instead of the hash.
//  As keys are sorted this could potentially give a performance boost due to better order across blocks
// todo document EVERYTHING: especially serialisation patterns to easily look them up later
// todo redo the storage system,
//  after syncing ~5_500 blocks the estimated storage at 100_000 blocks for tweaks alone,
//  will be somewhere around 40Gb
//  Additionally performance is getting worse
// todo investigate whether rpc parallel calls can speed up syncing
//  caution: currently the flow is synchronous and hence there is less complexity making parallel calls will change that
// todo include redundancy for when rpc calls are failing (probably due to networking issues in testing home environment)
// todo review all duplicate key error exemptions and raise to error/warn from debug
// todo remove unnecessary panics
func main() {
	defer func() {
		err := dblevel.HeadersDB.Close()
		if err != nil {
			common.ErrorLogger.Println(err)
		}
		err = dblevel.HeadersInvDB.Close()
		if err != nil {
			common.ErrorLogger.Println(err)
		}
		err = dblevel.FiltersDB.Close()
		if err != nil {
			common.ErrorLogger.Println(err)
		}
		err = dblevel.TweaksDB.Close()
		if err != nil {
			common.ErrorLogger.Println(err)
		}
		err = dblevel.UTXOsDB.Close()
		if err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	//log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	common.InfoLogger.Println("Program Started")

	// make sure everything is ready before we receive data

	// moved into go routine such that the interrupt signal will apply properly
	go func() {
		err := core.PreSyncHeaders()
		if err != nil {
			common.ErrorLogger.Fatalln(err)
			return
		}
		err = core.SyncChain()
		if err != nil {
			common.ErrorLogger.Fatalln(err)
			return
		}
		go core.CheckForNewBlockRoutine()
		go server.RunServer(&server.ApiHandler{})
	}()

	for true {
		select {
		case <-interrupt:
			common.InfoLogger.Println("Program interrupted")
			return
		}
	}
}

func openLevelDBConnections() {
	dblevel.HeadersDB = dblevel.OpenDBConnection(dblevel.DBPathHeaders)
	dblevel.HeadersInvDB = dblevel.OpenDBConnection(dblevel.DBPathHeadersInv)
	dblevel.FiltersDB = dblevel.OpenDBConnection(dblevel.DBPathFilters)
	dblevel.TweaksDB = dblevel.OpenDBConnection(dblevel.DBPathTweaks)
	dblevel.UTXOsDB = dblevel.OpenDBConnection(dblevel.DBPathUTXOs)
}
