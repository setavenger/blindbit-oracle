package main

import (
	"errors"
	"flag"
	"fmt"
	"path"

	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/core"
	"github.com/setavenger/blindbit-oracle/internal/dataexport"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/server"
	v2 "github.com/setavenger/blindbit-oracle/internal/server/v2"
)

var (
	displayVersion bool
	pruneOnStart   bool
	exportData     bool
	Version        = "0.0.0"
)

func init() {
	flag.StringVar(
		&config.BaseDirectory,
		"datadir",
		config.DefaultBaseDirectory,
		"Set the base directory for blindbit oracle. Default directory is ~/.blindbit-oracle",
	)
	flag.BoolVar(
		&displayVersion,
		"version",
		false,
		"show version of blindbit-oracle",
	)
	flag.BoolVar(
		&pruneOnStart,
		"reprune",
		false,
		"set this flag if you want to prune on startup",
	)
	flag.BoolVar(
		&exportData,
		"export-data",
		false,
		"export the databases",
	)
	flag.Parse()

	if displayVersion {
		// we only need the version for this
		return
	}

	config.SetDirectories() // todo a proper set settings function which does it all would be good to avoid several small function calls
	err := os.Mkdir(config.BaseDirectory, 0750)
	if err != nil && !errors.Is(err, os.ErrExist) {
		logging.L.Fatal().Err(err).Msg("error creating base directory")
	}

	logging.L.Info().Msgf("base directory %s", config.BaseDirectory)

	// load after loggers are instantiated
	config.LoadConfigs(path.Join(config.BaseDirectory, config.ConfigFileName))

	// create DB path
	err = os.Mkdir(config.DBPath, 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		logging.L.Err(err).Msg("error creating db path")
		os.Exit(1)
	}

	// open levelDB connections
	openLevelDBConnections()

	if config.CookiePath != "" {
		data, err := os.ReadFile(config.CookiePath)
		if err != nil {
			logging.L.Fatal().Err(err).Msg("error reading cookie file")
		}

		credentials := strings.Split(string(data), ":")
		if len(credentials) != 2 {
			logging.L.Fatal().Msg("cookie file is invalid")
		}
		config.RpcUser = credentials[0]
		config.RpcPass = credentials[1]
	}

	if config.RpcUser == "" {
		logging.L.Fatal().Msg("rpc user not set") // todo use cookie file to circumvent this requirement
	}

	if config.RpcPass == "" {
		logging.L.Fatal().Msg("rpc pass not set") // todo use cookie file to circumvent this requirement
	}
}

func main() {
	if displayVersion {
		fmt.Println("blindbit-oracle version:", Version) // using fmt because loggers are not initialised
		os.Exit(0)
	}
	defer logging.L.Info().Msg("Program shut down")
	defer dblevel.CloseDBs()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	logging.L.Info().Msg("Program Started")

	// make sure everything is ready before we receive data

	//todo create proper handling for exporting data

	if exportData {
		logging.L.Info().Msg("Exporting data")
		dataexport.ExportAll()
		// dataexport.ExportUTXOs(fmt.Sprintf("%s/export/utxos.csv", config.BaseDirectory))
		return
	}

	//moved into go routine such that the interrupt signal will apply properly
	go func() {
		if pruneOnStart {
			startPrune := time.Now()
			core.PruneAllUTXOs()
			logging.L.Info().Msgf("Pruning took: %s", time.Since(startPrune).String())
		}
		startSync := time.Now()
		err := core.PreSyncHeaders()
		if err != nil {
			logging.L.Fatal().Err(err).Msg("error pre-syncing headers")
			return
		}

		// so we can start fetching data while not fully synced. Requires headers to be synced to avoid grave errors.
		go server.RunServer(&server.ApiHandler{})

		// keep it optional for now
		if config.GRPCHost != "" {
			go v2.RunGRPCServer()
		}

		// todo buggy for sync catchup from 0, needs to be 1 or higher
		err = core.SyncChain()
		if err != nil {
			logging.L.Fatal().Err(err).Msg("error syncing chain")
			return
		}
		logging.L.Info().Msgf("Sync took: %s", time.Since(startSync).String())
		go core.CheckForNewBlockRoutine()

		// only call this if you need to reindex. It doesn't delete anything but takes a couple of minutes to finish
		//err := core.ReindexDustLimitsOnly()
		//if err != nil {
		//	logging.L.Err(err).Msg("error reindexing dust limits")
		//	return
		//}
	}()

	for {
		<-interrupt
		logging.L.Info().Msg("Program interrupted")
		return
	}
}

func openLevelDBConnections() {
	dblevel.HeadersDB = dblevel.OpenDBConnection(config.DBPathHeaders)
	dblevel.HeadersInvDB = dblevel.OpenDBConnection(config.DBPathHeadersInv)
	dblevel.NewUTXOsFiltersDB = dblevel.OpenDBConnection(config.DBPathFilters)
	dblevel.TweaksDB = dblevel.OpenDBConnection(config.DBPathTweaks)
	dblevel.TweakIndexDB = dblevel.OpenDBConnection(config.DBPathTweakIndex)
	dblevel.TweakIndexDustDB = dblevel.OpenDBConnection(config.DBPathTweakIndexDust)
	dblevel.UTXOsDB = dblevel.OpenDBConnection(config.DBPathUTXOs)
	dblevel.SpentOutpointsIndexDB = dblevel.OpenDBConnection(config.DBPathSpentOutpointsIndex)
	dblevel.SpentOutpointsFilterDB = dblevel.OpenDBConnection(config.DBPathSpentOutpointsFilter)
}
