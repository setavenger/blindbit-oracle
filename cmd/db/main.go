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

	// Lookup command flags
	lookupKeyType string
	lookupKey     string
	hexFormat     bool

	// Range command flags
	rangeKeyType    string
	rangeStartKey   string
	rangeLimit      int
	rangeShowValues bool

	// Prefix scan command flags
	prefixScanKeyType    string
	prefixScanKey        string
	prefixScanLimit      int
	prefixScanShowValues bool
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

	// Lookup command flags
	lookupCmd.Flags().StringVar(
		&lookupKeyType,
		"key-type",
		"",
		"Type of key to lookup: block-tx, tx, out, spend, ci-height, ci-block, tx-occur, tweaks-static, utxos-static, taproot-pubkey-filter, taproot-unspent-filter, taproot-spent-filter, compute-index",
	)
	lookupCmd.Flags().StringVar(
		&lookupKey,
		"key",
		"",
		"Key to lookup (hex encoded or raw depending on --hex flag)",
	)
	lookupCmd.Flags().BoolVar(
		&hexFormat,
		"hex",
		true,
		"Interpret key as hex encoded (default: true)",
	)

	// Range command flags
	rangeCmd.Flags().StringVar(
		&rangeKeyType,
		"key-type",
		"",
		"Type of keys to iterate: block-tx, tx, out, spend, ci-height, ci-block, tx-occur, tweaks-static, utxos-static, taproot-pubkey-filter, taproot-unspent-filter, taproot-spent-filter, compute-index",
	)
	rangeCmd.Flags().StringVar(
		&rangeStartKey,
		"start-key",
		"",
		"Starting key (hex encoded, optional - starts from beginning if not provided)",
	)
	rangeCmd.Flags().IntVar(
		&rangeLimit,
		"limit",
		10,
		"Maximum number of entries to iterate (default: 10)",
	)
	rangeCmd.Flags().BoolVar(
		&rangeShowValues,
		"show-values",
		false,
		"Show values along with keys (default: false)",
	)

	// Prefix scan command flags
	prefixScanCmd.Flags().StringVar(
		&prefixScanKeyType,
		"key-type",
		"",
		"Type of keys to scan: block-tx, tx, out, spend, ci-height, ci-block, tx-occur, tweaks-static, utxos-static, taproot-pubkey-filter, taproot-unspent-filter, taproot-spent-filter, compute-index",
	)
	prefixScanCmd.Flags().StringVar(
		&prefixScanKey,
		"prefix",
		"",
		"Prefix to scan for (hex encoded, e.g., '000D9D46' for height 892230)",
	)
	prefixScanCmd.Flags().IntVar(
		&prefixScanLimit,
		"limit",
		100,
		"Maximum number of entries to scan (default: 100)",
	)
	prefixScanCmd.Flags().BoolVar(
		&prefixScanShowValues,
		"show-values",
		false,
		"Show values along with keys (default: false)",
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

var lookupCmd = &cobra.Command{
	Use:   "lookup",
	Short: "Lookup a specific key in the database",
	Long: `Lookup a specific key in the database and return its value.
The key type determines the prefix byte, and the key data is provided as hex or raw bytes.

Examples:
  # Lookup a compute index key (height + txid)
  go run main.go lookup --key-type compute-index --key "0d000186a0a1b2c3d4e5f6..."

  # Lookup a transaction key (txid only)
  go run main.go lookup --key-type tx --key "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"

  # Lookup a CI height key (height only)
  go run main.go lookup --key-type ci-height --key "000186a0"`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if lookupKeyType == "" {
			return fmt.Errorf("key-type is required")
		}
		if lookupKey == "" {
			return fmt.Errorf("key is required")
		}

		fmt.Printf("Opening database at: %s\n", dbPath)
		fmt.Printf("Looking up %s key: %s\n", lookupKeyType, lookupKey)

		// Create database explorer
		explorer, err := NewDatabaseExplorer(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer explorer.Close()

		// Perform lookup
		value, err := explorer.LookupKey(lookupKeyType, lookupKey, hexFormat)
		if err != nil {
			return fmt.Errorf("error looking up key: %w", err)
		}

		if value == nil {
			fmt.Println("Key not found")
			return nil
		}

		fmt.Printf("Value: %x\n", value)
		return nil
	},
}

var rangeCmd = &cobra.Command{
	Use:   "range",
	Short: "Iterate through a range of keys in the database",
	Long: `Iterate through a range of keys in the database, starting from a specific key
and continuing for a specified number of entries. Useful for checking ranges of entries.

Examples:
  # Iterate through first 10 transaction keys
  go run main.go range --key-type tx --limit 10

  # Start from a specific key and iterate 5 entries
  go run main.go range --key-type tx --start-key "a1b2c3d4..." --limit 5

  # Show values along with keys
  go run main.go range --key-type ci-height --limit 5 --show-values

  # Iterate through compute index keys
  go run main.go range --key-type compute-index --limit 20`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rangeKeyType == "" {
			return fmt.Errorf("key-type is required")
		}

		fmt.Printf("Opening database at: %s\n", dbPath)
		fmt.Printf("Iterating through %s keys", rangeKeyType)
		if rangeStartKey != "" {
			fmt.Printf(" starting from: %s", rangeStartKey)
		}
		fmt.Printf(" (limit: %d)\n", rangeLimit)

		// Create database explorer
		explorer, err := NewDatabaseExplorer(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer explorer.Close()

		// Perform range iteration
		err = explorer.IterateKeyRange(rangeKeyType, rangeStartKey, rangeLimit, rangeShowValues)
		if err != nil {
			return fmt.Errorf("error iterating key range: %w", err)
		}

		return nil
	},
}

var prefixScanCmd = &cobra.Command{
	Use:   "prefix-scan",
	Short: "Scan keys with a specific prefix",
	Long: `Scan keys that start with a specific prefix. This is useful for finding all keys
that share a common prefix, such as all compute index keys for a specific height.

Examples:
  # Scan all compute index keys for height 892230 (0x000D9D46)
  go run main.go prefix-scan --key-type compute-index --prefix "000D9D46"

  # Scan with values shown
  go run main.go prefix-scan --key-type compute-index --prefix "000D9D46" --show-values

  # Scan with custom limit
  go run main.go prefix-scan --key-type compute-index --prefix "000D9D46" --limit 50`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if prefixScanKeyType == "" {
			return fmt.Errorf("key-type is required")
		}
		if prefixScanKey == "" {
			return fmt.Errorf("prefix is required")
		}

		fmt.Printf("Opening database at: %s\n", dbPath)
		fmt.Printf("Scanning %s keys with prefix: %s (limit: %d)\n", prefixScanKeyType, prefixScanKey, prefixScanLimit)

		// Create database explorer
		explorer, err := NewDatabaseExplorer(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer explorer.Close()

		// Perform prefix scan
		err = explorer.ScanKeyPrefix(prefixScanKeyType, prefixScanKey, prefixScanLimit, prefixScanShowValues)
		if err != nil {
			return fmt.Errorf("error scanning key prefix: %w", err)
		}

		return nil
	},
}

func main() {
	// Add subcommands
	rootCmd.AddCommand(countCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(listKeysCmd)
	rootCmd.AddCommand(lookupCmd)
	rootCmd.AddCommand(rangeCmd)
	rootCmd.AddCommand(prefixScanCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
