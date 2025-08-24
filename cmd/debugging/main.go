package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime"
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
	Version        = "0.0.0"
)

func init() {
	// hard code for now
	runtime.GOMAXPROCS(max(runtime.NumCPU()-4, 1))

	flag.StringVar(
		&config.BaseDirectory,
		"datadir",
		config.DefaultBaseDirectory,
		"Set the base directory for blindbit oracle. Default directory is ~/.blindbit-oracle",
	)
	flag.Parse()

	if displayVersion {
		// we only need the version for this
		return
	}

	// todo: a proper set settings function which does it all
	// avoid several small function calls
	config.SetDirectories()

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
	db, err := dbpebble.OpenDB()
	// db, err := database.OpenDB("")
	if err != nil {
		logging.L.Err(err).Msg("failed opening db")
		errChan <- err
		return
	}
	store := dbpebble.NewStore(db)

	//moved into go routine such that the interrupt signal will apply properly
	go func() {
		// so we can start fetching data while not fully synced.
		go server.RunServer(&server.ApiHandler{})

		// keep it optional for now
		if config.GRPCHost != "" {
			go v2.RunGRPCServer(store)
		}
	}()

	defer func() {
		err := db.Close()
		if err != nil {
			logging.L.Err(err).Msg("db close failed")
		}
		logging.L.Debug().Msg("db closed successfully")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// index builder
	go func() {
		builder := indexer.NewBuilder(ctx, store)
		// todo add non-sync option
		_ = builder

		err = builder.SyncBlocks(ctx, 240000, 240000)
		if err != nil {
			logging.L.Err(err).Msg("error indexing blocks")
			errChan <- err
			return
		}

		logging.L.Warn().Msg("initial sync done")

		err = store.FlushBatch()
		if err != nil {
			logging.L.Err(err).Msg("failed flushing batch")
			errChan <- err
			return
		}
	}()

	for {
		select {
		case <-interrupt:
			cancel()
			logging.L.Info().Msg("Program interrupted")
			return
		case err := <-errChan:
			cancel()
			logging.L.Err(err).Msg("program failed")
			return
		}
	}
}
