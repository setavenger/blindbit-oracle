package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime"

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
	datadir      string
	configFile   string
	skipPrecheck bool

	startHeight uint32
	endHeight   uint32
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
		&skipPrecheck,
		"skip-precheck",
		false,
		"Skip database integrity checks and other pre-checks (default: false)",
	)

	// sync flags
	syncCmd.Flags().Uint32Var(
		&startHeight,
		"start-height",
		0,
		"Start height",
	)
	syncCmd.Flags().Uint32Var(
		&endHeight,
		"end-height",
		0,
		"End height",
	)
}

// performDBIntegrityCheck performs database integrity check unless skipped by flag
// todo: debug it just tries syncing. Maybe when different ranges were synced in between. might be an issue mainly in dev settings. Could define a breka if gap greated x don't do the patch fixes. Alternatively do a concurrent sync so it goes at normal speed
func performDBIntegrityCheck(ctx context.Context, builder *indexer.Builder) error {
	if !skipPrecheck {
		logging.L.Info().Msg("Performing database integrity check...")
		err := builder.DBIntegrityCheck(ctx)
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

		if err = logging.EnableFileLogging(config.BaseDirectory, "debug.log"); err != nil {
			fmt.Println("base_dir:", config.BaseDirectory)
			logging.L.Fatal().Err(err).Msg("error setting log file")
		}

		logging.L.Info().Msgf("base directory %s", config.BaseDirectory)

		// Load config
		if configFile == "" {
			configFile = path.Join(config.BaseDirectory, config.ConfigFileName)
		}
		config.LoadConfigs(configFile)

		// Set CPU cores
		runtime.GOMAXPROCS(config.MaxCPUCores)
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync blockchain data (initial sync only)",
	Long: `Perform initial blockchain sync to the current tip. This command will:
- Sync all blocks from the first block to the current tip
- Not start continuous scanning or servers

Flags:
--skip-precheck flag to skip database integrity checks`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.L.Info().Msg("Starting initial blockchain sync...")

		db, err := dbpebble.OpenDB()
		if err != nil {
			return fmt.Errorf("failed opening db: %w", err)
		}

		store := dbpebble.NewStore(db)
		defer store.Close()

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		builder := indexer.NewBuilder(ctx, store)

		// Perform database integrity check unless skipped
		err = performDBIntegrityCheck(ctx, builder)
		if err != nil {
			return err
		}

		if startHeight > 0 && endHeight > 0 {
			err = builder.SyncBlocks(ctx, int64(startHeight), int64(endHeight))
			if err != nil {
				return fmt.Errorf("initial sync failed: %w", err)
			}
		} else {
			err = builder.InitialSyncToTip(ctx)
			if err != nil {
				return fmt.Errorf("initial sync failed: %w", err)
			}
		}

		// Perform database integrity check unless skipped
		err = performDBIntegrityCheck(ctx, builder)
		if err != nil {
			return err
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
--skip-precheck flag to skip database integrity checks (optional, default: false)
--start-height flag to start height (optional, default: 0)
--end-height flag to end height (optional, default: 0)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		logging.L.Info().Msg("Starting BlindBit Oracle service...")

		db, err := dbpebble.OpenDB()
		if err != nil {
			return fmt.Errorf("failed opening db: %w", err)
		}

		store := dbpebble.NewStore(db)
		defer store.Close()

		// Start servers
		go server.RunServer(server.NewHandler(store))

		if config.GRPCHost != "" {
			go v2.RunGRPCServer(store)
		}

		// Setup context and error handling
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		errChan := make(chan error, 1)

		// Start indexer
		go func() {
			builder := indexer.NewBuilder(ctx, store)

			// Perform database integrity check unless skipped
			err = performDBIntegrityCheck(ctx, builder)
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

// server only command no syncing only servers are set up
var serverCmd = &cobra.Command{
	Use:   "server-only",
	Short: "Run the full BlindBit Oracle service",
	Long: `Run the complete BlindBit Oracle service including:
- HTTP API server
- gRPC server (if configured)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.L.Info().Msg("Starting BlindBit Oracle service...")

		db, err := dbpebble.OpenDB()
		if err != nil {
			return fmt.Errorf("failed opening db: %w", err)
		}

		store := dbpebble.NewStore(db)
		defer store.Close()

		// Start servers
		go server.RunServer(server.NewHandler(store))

		if config.GRPCHost != "" {
			go v2.RunGRPCServer(store)
		}

		// Wait for interrupt or error
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		<-interrupt
		logging.L.Info().Msg("Service interrupted")

		return nil
	},
}

var acceleratorIndexesCmd = &cobra.Command{
	Use:   "accelerator-indexes",
	Short: "Build accelerator indexes for all blocks",
	Long: `Build accelerator indexes for all blocks in the database. This command will:
- Process all blocks from the first block to the current tip
- Create accelerator indexes like compute index for efficient scanning
- Not start continuous scanning or servers

Flags:
--start-height and --end-height to specify range`,
	RunE: func(cmd *cobra.Command, args []string) error {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		logging.L.Info().Msg("Starting accelerator index rebuild...")

		errChan := make(chan error)
		doneChan := make(chan struct{})

		go func() {
			defer close(doneChan)

			db, err := dbpebble.OpenDB()
			if err != nil {
				errChan <- fmt.Errorf("failed opening db: %w", err)
				return
			}

			store := dbpebble.NewStore(db)
			defer store.Close()

			_, syncTipHeight, err := store.GetChainTip()
			if err != nil {
				errChan <- fmt.Errorf("failed getting chain tip: %w", err)
				return
			}

			_, firstBlockHeight, err := store.FirstBlock()
			if err != nil {
				errChan <- fmt.Errorf("failed getting first block data: %w", err)
				return
			}

			endHeight = min(endHeight, syncTipHeight)

			// index starts where data is available
			startHeight = max(startHeight, firstBlockHeight)

			// Build Compute Index
			err = store.BuildComputeIndexByRange(startHeight, endHeight)
			if err != nil {
				errChan <- fmt.Errorf("accelerator indexing failed: %w", err)
				return
			}
		}()

		select {
		case <-interrupt:
			cancel()
			logging.L.Info().Msg("Accelerator index rebuild interrupted")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return err
		case <-doneChan:
			logging.L.Info().Msg("Accelerator index rebuild completed successfully")
			return nil
		}
	},
}

func init() {
	// accelerator indexes flags
	acceleratorIndexesCmd.Flags().Uint32Var(
		&startHeight,
		"start-height",
		0,
		"Start height",
	)
	acceleratorIndexesCmd.Flags().Uint32Var(
		&endHeight,
		"end-height",
		0,
		"End height",
	)
}

func main() {
	// Add subcommands
	rootCmd.AddCommand(acceleratorIndexesCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(serverCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
