package main

import (
	"flag"
	"fmt"
	"log"
	"path"

	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/core"
	"github.com/setavenger/blindbit-oracle/src/dataexport"
	"github.com/setavenger/blindbit-oracle/src/db/dblevel"
	"github.com/setavenger/blindbit-oracle/src/server"

	"os"
	"os/signal"
	"strings"
	"time"
)

var (
	displayVersion bool
	pruneOnStart   bool
	exportData     bool
	Version        = "0.0.0"
)

func init() {
	flag.StringVar(&common.BaseDirectory, "datadir", common.DefaultBaseDirectory, "Set the base directory for blindbit oracle. Default directory is ~/.blindbit-oracle")
	flag.BoolVar(&displayVersion, "version", false, "show version of blindbit-oracle")
	flag.BoolVar(&pruneOnStart, "reprune", false, "set this flag if you want to prune on startup")
	flag.BoolVar(&exportData, "export-data", false, "export the databases")
	flag.Parse()

	if displayVersion {
		// we only need the version for this
		return
	}
	common.SetDirectories() // todo a proper set settings function which does it all would be good to avoid several small function calls
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

	// file, err := os.OpenFile(fmt.Sprintf("%s/logs.log", common.LogsPath), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fileDebug, err := os.OpenFile(fmt.Sprintf("%s/logs-debug.log", common.LogsPath), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// multi := io.MultiWriter(file, os.Stdout)
	//multiDebug := io.MultiWriter(fileDebug, os.Stdout)

	// common.DebugLogger = log.New(fileDebug, "[DEBUG] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	// common.InfoLogger = log.New(multi, "[INFO] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	// common.WarningLogger = log.New(multi, "[WARNING] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	// common.ErrorLogger = log.New(multi, "[ERROR] ", log.Ldate|log.Lmicroseconds|log.Llongfile|log.Lmsgprefix)

	common.InfoLogger.Println("base directory", common.BaseDirectory)

	// load after loggers are instantiated
	common.LoadConfigs(path.Join(common.BaseDirectory, common.ConfigFileName))

	// create DB path
	err = os.Mkdir(common.DBPath, 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		common.ErrorLogger.Println(err)
		panic(err)
	}

	// open levelDB connections
	openLevelDBConnections()

	if common.CookiePath != "" {
		data, err := os.ReadFile(common.CookiePath)
		if err != nil {
			panic(err)
		}

		credentials := strings.Split(string(data), ":")
		if len(credentials) != 2 {
			panic("cookie file is invalid")
		}
		common.RpcUser = credentials[0]
		common.RpcPass = credentials[1]
	}

	if common.RpcUser == "" {
		panic("rpc user not set") // todo use cookie file to circumvent this requirement
	}

	if common.RpcPass == "" {
		panic("rpc pass not set") // todo use cookie file to circumvent this requirement
	}
}

func main() {
	if displayVersion {
		fmt.Println("blindbit-oracle version:", Version) // using fmt because loggers are not initialised
		os.Exit(0)
	}
	defer common.InfoLogger.Println("Program shut down")
	defer closeDBs()

	//log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	common.InfoLogger.Println("Program Started")

	// make sure everything is ready before we receive data

	//todo create proper handling for exporting data

	if exportData {
		common.InfoLogger.Println("Exporting data")
		dataexport.ExportUTXOs(fmt.Sprintf("%s/export/utxos.csv", common.BaseDirectory))
		return
	}

	//moved into go routine such that the interrupt signal will apply properly
	go func() {
		if pruneOnStart {
			startPrune := time.Now()
			core.PruneAllUTXOs()
			common.InfoLogger.Printf("Pruning took: %s", time.Since(startPrune).String())
		}
		startSync := time.Now()
		err := core.PreSyncHeaders()
		if err != nil {
			common.ErrorLogger.Fatalln(err)
			return
		}

		// so we can start fetching data while not fully synced. Requires headers to be synced to avoid grave errors.
		go server.RunServer(&server.ApiHandler{})

		// todo buggy for sync catchup from 0, needs to be 1 or higher
		err = core.SyncChain()
		if err != nil {
			common.ErrorLogger.Fatalln(err)
			return
		}
		common.InfoLogger.Printf("Sync took: %s", time.Since(startSync).String())
		go core.CheckForNewBlockRoutine()

		// only call this if you need to reindex. It doesn't delete anything but takes a couple of minutes to finish
		//err := core.ReindexDustLimitsOnly()
		//if err != nil {
		//	common.ErrorLogger.Fatalln(err)
		//	return
		//}

	}()

	for {
		<-interrupt
		common.InfoLogger.Println("Program interrupted")
		return
	}
}

func openLevelDBConnections() {
	dblevel.HeadersDB = dblevel.OpenDBConnection(common.DBPathHeaders)
	dblevel.HeadersInvDB = dblevel.OpenDBConnection(common.DBPathHeadersInv)
	dblevel.NewUTXOsFiltersDB = dblevel.OpenDBConnection(common.DBPathFilters)
	dblevel.TweaksDB = dblevel.OpenDBConnection(common.DBPathTweaks)
	dblevel.TweakIndexDB = dblevel.OpenDBConnection(common.DBPathTweakIndex)
	dblevel.TweakIndexDustDB = dblevel.OpenDBConnection(common.DBPathTweakIndexDust)
	dblevel.UTXOsDB = dblevel.OpenDBConnection(common.DBPathUTXOs)
	dblevel.SpentOutpointsIndexDB = dblevel.OpenDBConnection(common.DBPathSpentOutpointsIndex)
	dblevel.SpentOutpointsFilterDB = dblevel.OpenDBConnection(common.DBPathSpentOutpointsFilter)
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
	err = dblevel.NewUTXOsFiltersDB.Close()
	if err != nil {
		common.ErrorLogger.Println(err)
	}
	err = dblevel.TweaksDB.Close()
	if err != nil {
		common.ErrorLogger.Println(err)
	}
	err = dblevel.TweakIndexDB.Close()
	if err != nil {
		common.ErrorLogger.Println(err)
	}
	err = dblevel.TweakIndexDustDB.Close()
	if err != nil {
		common.ErrorLogger.Println(err)
	}
	err = dblevel.UTXOsDB.Close()
	if err != nil {
		common.ErrorLogger.Println(err)
	}
	err = dblevel.SpentOutpointsIndexDB.Close()
	if err != nil {
		common.ErrorLogger.Println(err)
	}
	err = dblevel.SpentOutpointsFilterDB.Close()
	if err != nil {
		common.ErrorLogger.Println(err)
	}

	common.InfoLogger.Println("DBs closed")
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
