package main

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/core"
	"SilentPaymentAppBackend/src/dataexport"
	"SilentPaymentAppBackend/src/db/dblevel"
	"SilentPaymentAppBackend/src/server"
	"fmt"
	"io"
	"log"
	//_ "net/http/pprof" // Import for side effects: registers pprof handlers with the default mux.
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

func init() {
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:6060", nil))
	//}()
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
		var catchUpRawConv uint64
		catchUpRawConv, err = strconv.ParseUint(catchUpRaw, 10, 32)
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

	//todo create proper handling for exporting data
	//exportAll()

	//moved into go routine such that the interrupt signal will apply properly
	go func() {
		startSync := time.Now()
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
		common.InfoLogger.Printf("Sync took: %s", time.Now().Sub(startSync).String())
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
	dblevel.TweakIndexDB = dblevel.OpenDBConnection(dblevel.DBPathTweakIndex)
	dblevel.UTXOsDB = dblevel.OpenDBConnection(dblevel.DBPathUTXOs)
}

func exportAll() {
	// todo manage memory better, bloats completely during export
	common.InfoLogger.Println("Exporting data")
	timestamp := time.Now()

	err := dataexport.ExportUTXOs(fmt.Sprintf("./data-export/utxos-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	common.InfoLogger.Println("Finished UTXOs")

	err = dataexport.ExportFilters(fmt.Sprintf("./data-export/filters-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	common.InfoLogger.Println("Finished Filters")

	err = dataexport.ExportTweaks(fmt.Sprintf("./data-export/tweaks-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	common.InfoLogger.Println("Finished Tweaks")

	err = dataexport.ExportTweakIndices(fmt.Sprintf("./data-export/tweak-indices-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	common.InfoLogger.Println("Finished Tweak Indices")

	err = dataexport.ExportHeadersInv(fmt.Sprintf("./data-export/headers-inv-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	common.InfoLogger.Println("Finished HeadersInv")

	common.InfoLogger.Println("All exported")
	os.Exit(0)
}
