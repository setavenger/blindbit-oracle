package main

import (
	"fmt"
	"os"
	"path"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/spf13/cobra"
)

var (
	Version = "0.0.0" // todo: LD flags etc. to setup correctly and add git hash

	// Global flags
	datadir    string
	configFile string
	dbPath     string

	// Count command flags
	startHeight uint32
	endHeight   uint32
	keyType     string
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
	rootCmd.PersistentFlags().StringVar(
		&dbPath,
		"db",
		"",
		"Path to the pebble database directory (default: datadir/pebbledb/db)",
	)

	// Count command flags
	countCmd.Flags().Uint32Var(
		&startHeight,
		"start-height",
		0,
		"Start height for key counting (required for height-based keys)",
	)
	countCmd.Flags().Uint32Var(
		&endHeight,
		"end-height",
		0,
		"End height for key counting (required for height-based keys)",
	)
	countCmd.Flags().StringVar(
		&keyType,
		"key-type",
		"compute-index",
		"Type of keys to count: block-tx, tx, out, spend, ci-height, ci-block, tx-occur, tweaks-static, utxos-static, taproot-pubkey-filter, taproot-unspent-filter, taproot-spent-filter, compute-index",
	)
}

var rootCmd = &cobra.Command{
	Use:   "db-explorer",
	Short: "BlindBit Oracle Database Explorer",
	Long: `BlindBit Oracle Database Explorer provides tools to explore and analyze
the pebble database used by the BlindBit Oracle service.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set directories and initialize config
		config.BaseDirectory = datadir
		config.SetDirectories()

		logging.L.Info().Msgf("base directory %s", config.BaseDirectory)

		// Load config
		if configFile == "" {
			configFile = path.Join(config.BaseDirectory, config.ConfigFileName)
		}
		config.LoadConfigs(configFile)

		// Set database path if not provided
		if dbPath == "" {
			logging.L.Fatal().Msg("db path not provided")
			// dbPath = path.Join(config.BaseDirectory, "pebbledb", "db")
		}
	},
}

var countCmd = &cobra.Command{
	Use:   "count",
	Short: "Count keys in the database",
	Long: `Count keys of a specific type in the database. For height-based keys,
you must specify both start-height and end-height. For other key types,
height parameters are ignored.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Opening database at: %s\n", dbPath)
		fmt.Printf("Counting %s keys", keyType)

		// Create database explorer
		explorer, err := NewDatabaseExplorer(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer explorer.Close()

		// Check if this is a height-based key type
		heightBasedKeys := map[string]bool{
			"ci-height":              true,
			"compute-index":          true,
			"tweaks-static":          true,
			"utxos-static":           true,
			"taproot-pubkey-filter":  true,
			"taproot-unspent-filter": true,
			"taproot-spent-filter":   true,
		}

		if heightBasedKeys[keyType] {
			if startHeight == 0 || endHeight == 0 {
				return fmt.Errorf("start-height and end-height are required for key type: %s", keyType)
			}
			if startHeight > endHeight {
				return fmt.Errorf("start-height must be less than or equal to end-height")
			}
			fmt.Printf(" from height %d to %d\n", startHeight, endHeight)
		} else {
			fmt.Println()
		}

		// Count keys
		count, err := explorer.CountKeysByType(keyType, startHeight, endHeight)
		if err != nil {
			return fmt.Errorf("error counting keys: %w", err)
		}

		fmt.Printf("Found %d %s keys", count, keyType)
		if heightBasedKeys[keyType] {
			fmt.Printf(" in height range %d-%d", startHeight, endHeight)
		}
		fmt.Println()
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show database information",
	Long: `Show comprehensive database information including:
- Height range (min/max blocks)
- Key type counts by prefix
- Database metrics (memtable size, cache size, WAL info, etc.)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Opening database at: %s\n", dbPath)

		// Create database explorer
		explorer, err := NewDatabaseExplorer(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer explorer.Close()

		// Print database information
		if err := explorer.PrintDatabaseInfo(); err != nil {
			return fmt.Errorf("error printing database info: %w", err)
		}

		return nil
	},
}

var listKeysCmd = &cobra.Command{
	Use:   "list-keys",
	Short: "List all key types in the database",
	Long: `List all key types present in the database with their counts.
This provides an overview of what data is stored in the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Opening database at: %s\n", dbPath)

		// Create database explorer
		explorer, err := NewDatabaseExplorer(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer explorer.Close()

		// Print key type summary
		if err := explorer.PrintKeyTypeSummary(); err != nil {
			return fmt.Errorf("error printing key type summary: %w", err)
		}

		return nil
	},
}

func main() {
	// Add subcommands
	rootCmd.AddCommand(countCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(listKeysCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
