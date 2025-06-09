package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"path"
	"strings"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/indexer/bitcoinkernel"
)

var (
	displayVersion bool
	pruneOnStart   bool
	exportData     bool
	Version        = "0.0.0"
)

var (
	height        uint64
	kernelDatadir string

	syncHeightStart uint64
	syncHeightEnd   uint64
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
	flag.Uint64Var(
		&height,
		"height",
		0,
		"height to start the indexer at",
	)
	flag.StringVar(
		&kernelDatadir,
		"kernel-datadir",
		"",
		"path to the bitcoin core datadir",
	)
	flag.Uint64Var(
		&syncHeightStart,
		"sync-height-start",
		0,
		"height to sync to tip from",
	)
	flag.Uint64Var(
		&syncHeightEnd,
		"sync-height-end",
		0,
		"height to sync to tip from",
	)
	flag.Parse()

	if displayVersion {
		// we only need the version for this
		return
	}

	bitcoinkernel.SetDatadir(kernelDatadir)

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
	if height > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		err := bitcoinkernel.SyncHeight(ctx, uint32(height))
		if err != nil {
			logging.L.Fatal().Err(err).Msg("error syncing height")
		}
	}

	if syncHeightStart > 0 {
		var endHeight *uint32
		if syncHeightEnd > 0 {
			syncHeightEndU32 := uint32(syncHeightEnd)
			endHeight = &syncHeightEndU32
		}
		err := bitcoinkernel.SyncToTipFromHeight(context.Background(), uint32(syncHeightStart), endHeight)
		if err != nil {
			logging.L.Fatal().Err(err).Msg("error syncing to tip from height")
		}
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
