package main

import (
	"context"
	"errors"
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
	"github.com/spf13/cobra"
)

var (
	Version = "0.0.0" //todo LD flags etc. to setup correctly and add git hash

	// Global flags
	datadir       string
	configFile    string
	reindexStatic bool
	skipPrecheck  bool
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(
		&datadir,
		"datadir",
		config.DefaultBaseDirectory,
		"Set the base directory for blindbit oracle. Default directory is ~/.blindbit-oracle",
	)
	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Path to config file (default: datadir/blindbit.toml)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&reindexStatic,
		"reindex-static",
		false,
		"Force rebuild of static indexes (default: false)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&skipPrecheck,
		"skip-precheck",
		false,
		"Skip database integrity checks and other pre-checks (default: false)",
	)
}

// performDBIntegrityCheck performs database integrity check unless skipped by flag
func performDBIntegrityCheck(builder *indexer.Builder) error {
	if !skipPrecheck {
		logging.L.Info().Msg("Performing database integrity check...")
		err := builder.DBIntegrityCheck()
		if err != nil {
			return fmt.Errorf("db integrity check failed: %w", err)
		}
		logging.L.Info().Msg("Database integrity check completed successfully")
	} else {
		logging.L.Info().Msg("Skipping database integrity check (--skip-precheck flag set)")
	}
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "blindbit-oracle",
	Short: "BlindBit Oracle - Bitcoin UTXO indexing and scanning service",
	Long: `BlindBit Oracle is a Bitcoin UTXO indexing and scanning service that provides
efficient blockchain data processing and API access for Bitcoin applications.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set directories and initialize config
		config.BaseDirectory = datadir
		config.SetDirectories()

		// Create base directory
		err := os.Mkdir(config.BaseDirectory, 0750)
		if err != nil && !errors.Is(err, os.ErrExist) {
			logging.L.Fatal().Err(err).Msg("error creating base directory")
		}

		logging.L.Info().Msgf("base directory %s", config.BaseDirectory)

		// Load config
		if configFile == "" {
			configFile = path.Join(config.BaseDirectory, config.ConfigFileName)
		}
		config.LoadConfigs(configFile)

		// Set CPU cores
		runtime.GOMAXPROCS(config.MaxCPUCores)

		// Create DB path
		err = os.Mkdir(config.DBPath, 0750)
		if err != nil && !strings.Contains(err.Error(), "file exists") {
			logging.L.Fatal().Err(err).Msg("error creating db path")
		}
	},
}

var staticIndexesCmd = &cobra.Command{
	Use:   "static-indexes",
	Short: "Build static indexes for all blocks",
	Long: `Build static indexes for all blocks in the database. This command will:
- Process all blocks from the first block to the current tip
- Create static indexes for tweaks and outputs
- Not start continuous scanning or servers

Flags:
--reindex-static flag to force rebuild of existing indexes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logging.L.Info().Msg("Starting static index rebuild...")

		errChan := make(chan error)
		doneChan := make(chan struct{})

		go func() {
			defer close(doneChan)

			db, err := dbpebble.OpenDB()
			if err != nil {
				errChan <- fmt.Errorf("failed opening db: %w", err)
				return
			}
			defer db.Close()

			store := dbpebble.NewStore(db)
			defer store.FlushBatch(true)

			// todo: should be in an indexer.Builder
			err = store.BuildStaticIndexing(reindexStatic)
			if err != nil {
				errChan <- fmt.Errorf("static indexing failed: %w", err)
				return
			}
		}()

		select {
		case <-interrupt:
			cancel()
			logging.L.Info().Msg("Static index rebuild interrupted")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return err
		case <-doneChan:
			logging.L.Info().Msg("Static index rebuild completed successfully")
			return nil
		}
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync blockchain data (initial sync only)",
	Long: `Perform initial blockchain sync to the current tip. This command will:
- Sync all blocks from the first block to the current tip
- Not start continuous scanning or servers
- Not rebuild static indexes

Flags:
--skip-precheck flag to skip database integrity checks`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.L.Info().Msg("Starting initial blockchain sync...")

		db, err := dbpebble.OpenDB()
		if err != nil {
			return fmt.Errorf("failed opening db: %w", err)
		}
		defer db.Close()

		store := dbpebble.NewStore(db)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		builder := indexer.NewBuilder(ctx, store)

		// Perform database integrity check unless skipped
		err = performDBIntegrityCheck(builder)
		if err != nil {
			return err
		}

		err = builder.InitialSyncToTip(ctx)
		if err != nil {
			return fmt.Errorf("initial sync failed: %w", err)
		}

		logging.L.Info().Msg("Initial blockchain sync completed successfully")
		return nil
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the full BlindBit Oracle service",
	Long: `Run the complete BlindBit Oracle service including:
- Initial blockchain sync
- Continuous scanning for new blocks
- HTTP API server
- gRPC server (if configured)

Flags:
--reindex-static flag to force rebuild of static indexes (optional, default: false)
--skip-precheck flag to skip database integrity checks (optional, default: false)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.L.Info().Msg("Starting BlindBit Oracle service...")

		db, err := dbpebble.OpenDB()
		if err != nil {
			return fmt.Errorf("failed opening db: %w", err)
		}
		defer db.Close()

		store := dbpebble.NewStore(db)

		// Start servers
		go server.RunServer(&server.ApiHandler{})

		if config.GRPCHost != "" {
			go v2.RunGRPCServer(store)
		}

		// Setup context and error handling
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		errChan := make(chan error, 1)

		// Start indexer
		go func() {
			builder := indexer.NewBuilder(ctx, store)

			// Perform database integrity check unless skipped
			err = performDBIntegrityCheck(builder)
			if err != nil {
				errChan <- err
				return
			}

			// Do initial sync
			err = builder.InitialSyncToTip(ctx)
			if err != nil {
				errChan <- fmt.Errorf("failed initial sync: %w", err)
				return
			}
			logging.L.Info().Msg("initial sync done")

			// Build static indexes if requested
			if reindexStatic {
				logging.L.Info().Msg("Building static indexes...")
				err = store.BuildStaticIndexing(true)
				if err != nil {
					errChan <- fmt.Errorf("static indexing failed: %w", err)
					return
				}
				logging.L.Info().Msg("static indexes built")
			} else {
				logging.L.Info().Msg("Skipping static index build (--reindex-static flag not set)")
			}

			// Start continuous sync
			err = builder.ContinuousSync(ctx)
			if err != nil {
				errChan <- fmt.Errorf("continuous sync failed: %w", err)
				return
			}
		}()

		// Wait for interrupt or error
		for {
			select {
			case <-interrupt:
				cancel()
				logging.L.Info().Msg("Service interrupted")
				return nil
			case err := <-errChan:
				cancel()
				return err
			}
		}
	},
}

func main() {
	// Add subcommands
	rootCmd.AddCommand(staticIndexesCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(runCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
