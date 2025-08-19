package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path"

	"os"
	"os/signal"
	"strings"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
	"github.com/setavenger/blindbit-oracle/internal/indexer"
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
		logging.L.Fatal().Err(err).Msg("error creating db path")
	}

	// open levelDB connections
	// openLevelDBConnections()

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

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	logging.L.Info().Msg("Program Started")

	errChan := make(chan error)

	//moved into go routine such that the interrupt signal will apply properly
	go func() {
		// so we can start fetching data while not fully synced.
		go server.RunServer(&server.ApiHandler{})

		// keep it optional for now
		if config.GRPCHost != "" {
			go v2.RunGRPCServer()
		}
	}()

	db, err := dbpebble.OpenDB()
	// db, err := database.OpenDB("")
	if err != nil {
		logging.L.Err(err).Msg("failed opening db")
		errChan <- err
		return
	}

	defer func() {
		err := db.Close()
		if err != nil {
			logging.L.Err(err).Msg("db close failed")
		}
		logging.L.Debug().Msg("db closed successfully")
	}()

	// index builder
	go func() {
		// err = database.DropIndexesForIBD(context.Background(), db)
		// if err != nil {
		// 	logging.L.Err(err).Msg("failed to drop indexes")
		// 	errChan <- err
		// }

		builder := indexer.NewBuilderPebble(db)
		// builder := indexer.NewBuilder(db)

		// ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		// defer cancel()

		err = builder.SyncBlocks(context.Background(), 1, 260_000)
		if err != nil {
			logging.L.Err(err).Msg("error indexing blocks")
			errChan <- err
			return
		}
	}()

	for {
		select {
		case <-interrupt:
			logging.L.Info().Msg("Program interrupted")
			return
		case err := <-errChan:
			logging.L.Err(err).Msg("program failed")
			return
		}
	}
}

// func openLevelDBConnections() {
// 	dblevel.HeadersDB = dblevel.OpenDBConnection(config.DBPathHeaders)
// 	dblevel.HeadersInvDB = dblevel.OpenDBConnection(config.DBPathHeadersInv)
// 	dblevel.NewUTXOsFiltersDB = dblevel.OpenDBConnection(config.DBPathFilters)
// 	dblevel.TweaksDB = dblevel.OpenDBConnection(config.DBPathTweaks)
// 	dblevel.TweakIndexDB = dblevel.OpenDBConnection(config.DBPathTweakIndex)
// 	dblevel.TweakIndexDustDB = dblevel.OpenDBConnection(config.DBPathTweakIndexDust)
// 	dblevel.UTXOsDB = dblevel.OpenDBConnection(config.DBPathUTXOs)
// 	dblevel.SpentOutpointsIndexDB = dblevel.OpenDBConnection(config.DBPathSpentOutpointsIndex)
// 	dblevel.SpentOutpointsFilterDB = dblevel.OpenDBConnection(config.DBPathSpentOutpointsFilter)
// }
