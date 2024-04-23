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
	// for profiling or testing iirc
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:6060", nil))
	//}()

	baseDir := os.Getenv("BASE_DIRECTORY")
	if baseDir == "" {
		panic("base directory not set")
	}
	common.BaseDirectory = baseDir
	common.SetDirectories() // todo a proper set settings function which does it all would be good to avoid several small function calls

	tweaksOnly := os.Getenv("TWEAKS_ONLY")
	if tweaksOnly != "" {
		tweaksOnlyInt, err := strconv.ParseInt(tweaksOnly, 10, 64)
		if err != nil {
			panic(err)
		}
		switch tweaksOnlyInt {
		case 0:
			common.TweaksOnly = false
		case 1:
			common.TweaksOnly = true
		default:
			panic(fmt.Sprintf("only 0 or 1 allowed received %d", tweaksOnlyInt))
		}
	} else {
		common.TweaksOnly = false
	}

	err := os.Mkdir(common.BaseDirectory, 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		fmt.Println(err.Error())
		log.Fatal(err)
	}

	err = os.Mkdir(common.LogsPath, 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		fmt.Println(err.Error())
		log.Fatal(err)
	}

	file, err := os.OpenFile(fmt.Sprintf("%s/logs/logs-%s.txt", common.BaseDirectory, time.Now()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	fileDebug, err := os.OpenFile(fmt.Sprintf("%s/logs/logs-debug-%s.txt", common.BaseDirectory, time.Now()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
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
	err = os.Mkdir(common.DBPath, 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		common.ErrorLogger.Println(err)
		panic(err)
	}

	// load env vars
	syncStartHeightStr := os.Getenv("SYNC_START_HEIGHT")
	if syncStartHeightStr != "" {
		var syncStartHeightConv uint64
		syncStartHeightConv, err = strconv.ParseUint(syncStartHeightStr, 10, 32)
		if err != nil {
			common.ErrorLogger.Println(err)
			panic(err)
		}
		common.SyncStartHeight = uint32(syncStartHeightConv)
	} else {
		panic("SYNC_START_HEIGHT not set")
	}
	maxParallelRequestsStr := os.Getenv("MAX_PARALLEL_REQUESTS")
	if maxParallelRequestsStr != "" {
		var maxParallelRequestsConv uint64
		maxParallelRequestsConv, err = strconv.ParseUint(maxParallelRequestsStr, 10, 16)
		if err != nil {
			common.ErrorLogger.Println(err)
			panic(err)
		}
		common.MaxParallelRequests = uint16(maxParallelRequestsConv)
	}

	maxParallelTweakComputationsStr := os.Getenv("MAX_PARALLEL_TWEAK_COMPUTATIONS")
	if maxParallelTweakComputationsStr != "" {
		var maxParallelTweakComputationsConv int64
		maxParallelTweakComputationsConv, err = strconv.ParseInt(maxParallelTweakComputationsStr, 10, 64)
		if err != nil {
			common.ErrorLogger.Println(err)
			panic(err)
		}
		common.MaxParallelTweakComputations = int(maxParallelTweakComputationsConv)
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
	defer closeDBs()

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
		// todo buggy for sync catchup from 0, needs to be 1 or higher
		err = core.SyncChain()
		if err != nil {
			common.ErrorLogger.Fatalln(err)
			return
		}
		common.InfoLogger.Printf("Sync took: %s", time.Now().Sub(startSync).String())
		go core.CheckForNewBlockRoutine()

		// only call this if you need to reindex. It doesn't delete anything but takes a couple of minutes to finish
		//err := core.ReindexDustLimitsOnly()
		//if err != nil {
		//	common.ErrorLogger.Fatalln(err)
		//	return
		//}

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
	dblevel.HeadersDB = dblevel.OpenDBConnection(common.DBPathHeaders)
	dblevel.HeadersInvDB = dblevel.OpenDBConnection(common.DBPathHeadersInv)
	dblevel.FiltersDB = dblevel.OpenDBConnection(common.DBPathFilters)
	dblevel.TweaksDB = dblevel.OpenDBConnection(common.DBPathTweaks)
	dblevel.TweakIndexDB = dblevel.OpenDBConnection(common.DBPathTweakIndex)
	dblevel.UTXOsDB = dblevel.OpenDBConnection(common.DBPathUTXOs)
}

func closeDBs() {
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
