# Blindbit Oracle Database Explorer

A command-line tool for exploring and analyzing the Blindbit Oracle pebble database using Cobra/Viper CLI framework.

## Features

- **Count Keys by Type**: Count keys of specific types in height ranges
- **Database Information**: Show comprehensive database statistics and key type summaries
- **Height Range Analysis**: Get min/max height ranges in the database
- **Multiple Key Types**: Support for all database key types and prefixes

## Commands

### Count Keys

Count keys of a specific type, optionally within a height range:

```bash
# Count compute index keys from height 100000 to 100100
go run main.go count --key-type compute-index --start-height 100000 --end-height 100100

# Count all transaction keys (no height range needed)
go run main.go count --key-type tx

# Count CI height keys in range
go run main.go count --key-type ci-height --start-height 500000 --end-height 500500
```

### Show Database Information

Get comprehensive database statistics:

```bash
# Show database info using default path
go run main.go info

# Show database info using custom path
go run main.go --db /path/to/db info
```

### List Key Types

List all key types present in the database:

```bash
# List all key types with counts
go run main.go list-keys
```

### Lookup Key

Lookup a specific key in the database and return its value:

```bash
# Lookup a compute index key (height + txid)
go run main.go lookup --key-type compute-index --key "0d000186a0a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef"

# Lookup a transaction key (txid only)
go run main.go lookup --key-type tx --key "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"

# Lookup a CI height key (height only)
go run main.go lookup --key-type ci-height --key "000186a0"

# Lookup with raw bytes instead of hex
go run main.go lookup --key-type tx --key "raw_key_data" --hex=false
```

### Range Iteration

Iterate through a range of keys, starting from a specific key and continuing for a specified number of entries:

```bash
# Iterate through first 10 transaction keys
go run main.go range --key-type tx --limit 10

# Start from a specific key and iterate 5 entries
go run main.go range --key-type tx --start-key "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456" --limit 5

# Show values along with keys
go run main.go range --key-type ci-height --limit 5 --show-values

# Iterate through compute index keys
go run main.go range --key-type compute-index --limit 20

# Start from a specific compute index key
go run main.go range --key-type compute-index --start-key "000186a0a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef" --limit 10
```

### Prefix Scan

Scan keys that start with a specific prefix. This is useful for finding all keys that share a common prefix, such as all compute index keys for a specific height:

```bash
# Scan all compute index keys for height 892230 (0x000D9D46)
go run main.go prefix-scan --key-type compute-index --prefix "000D9D46"

# Scan with values shown
go run main.go prefix-scan --key-type compute-index --prefix "000D9D46" --show-values

# Scan with custom limit
go run main.go prefix-scan --key-type compute-index --prefix "000D9D46" --limit 50

# Scan CI height keys for a specific height
go run main.go prefix-scan --key-type ci-height --prefix "000186a0"
```

### Help

Show help information:

```bash
# General help
go run main.go --help

# Command-specific help
go run main.go count --help
go run main.go info --help
```

## Global Options

- `--datadir string`: Set the base directory for blindbit oracle (default: `~/.blindbit-oracle`)
- `--config string`: Path to config file (default: `datadir/blindbit.toml`)
- `--db string`: Path to the pebble database directory (default: `datadir/pebbledb/db`)

## Count Command Options

- `--key-type string`: Type of keys to count (default: `compute-index`)
- `--start-height uint32`: Start height for key counting (required for height-based keys)
- `--end-height uint32`: End height for key counting (required for height-based keys)

## Lookup Command Options

- `--key-type string`: Type of key to lookup (required)
- `--key string`: Key to lookup (required)
- `--hex bool`: Interpret key as hex encoded (default: `true`)

## Range Command Options

- `--key-type string`: Type of keys to iterate (required)
- `--start-key string`: Starting key (hex encoded, optional - starts from beginning if not provided)
- `--limit int`: Maximum number of entries to iterate (default: `10`)
- `--show-values bool`: Show values along with keys (default: `false`)

## Prefix Scan Command Options

- `--key-type string`: Type of keys to scan (required)
- `--prefix string`: Prefix to scan for (hex encoded, required)
- `--limit int`: Maximum number of entries to scan (default: `100`)
- `--show-values bool`: Show values along with keys (default: `false`)

## Examples

```bash
# Count compute index keys in a small range
go run main.go count --key-type compute-index --start-height 833000 --end-height 833010

# Count compute index keys in a larger range
go run main.go count --key-type compute-index --start-height 100000 --end-height 200000

# Count all transaction keys
go run main.go count --key-type tx

# Count CI height keys
go run main.go count --key-type ci-height --start-height 500000 --end-height 500100

# Get database overview
go run main.go info

# List all key types
go run main.go list-keys

# Use custom database location
go run main.go --db /custom/path/to/pebbledb/db info

# Use custom datadir
go run main.go --datadir /custom/blindbit/dir count --key-type compute-index --start-height 100000 --end-height 100100

# Lookup examples
go run main.go lookup --key-type tx --key "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"
go run main.go lookup --key-type ci-height --key "000186a0"
go run main.go lookup --key-type compute-index --key "0d000186a0a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef"

# Range iteration examples
go run main.go range --key-type tx --limit 10
go run main.go range --key-type ci-height --limit 5 --show-values
go run main.go range --key-type compute-index --start-key "000186a0a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef" --limit 10

# Prefix scan examples
go run main.go prefix-scan --key-type compute-index --prefix "000D9D46"
go run main.go prefix-scan --key-type ci-height --prefix "000186a0" --show-values
go run main.go prefix-scan --key-type compute-index --prefix "000D9D46" --limit 50
```

## Supported Key Types

The database contains several types of keys that can be counted:

### Height-Based Key Types (require --start-height and --end-height)

- **`compute-index`**: Compute index keys storing tweak and output data
- **`ci-height`**: Chain index height mappings (height → block hash)
- **`tweaks-static`**: Static tweak data by block
- **`utxos-static`**: Static UTXO data by block
- **`taproot-pubkey-filter`**: Taproot pubkey filters by block
- **`taproot-unspent-filter`**: Taproot unspent filters by block
- **`taproot-spent-filter`**: Taproot spent filters by block

### Non-Height-Based Key Types (height parameters ignored)

- **`block-tx`**: Block to transaction mappings
- **`tx`**: Transaction records with tweak data
- **`out`**: Transaction outputs
- **`spend`**: Spend events
- **`ci-block`**: Chain index block mappings (block hash → height)
- **`tx-occur`**: Transaction occurrence records

## Key Lookup Details

The lookup command allows you to retrieve values for specific keys in the database. The key format depends on the key type:

### Key Formats by Type

The lookup command automatically adds the prefix byte for the specified key type. You can provide either:

1. **Data portion only** (recommended): The key data without the prefix byte
2. **Full key**: The complete key including the prefix byte

**Data formats by type:**
- **`tx`**: Transaction ID (32 bytes hex)
- **`out`**: Transaction ID + output index (32 + 4 bytes hex)
- **`spend`**: Previous transaction ID + output index + block hash (32 + 4 + 32 bytes hex)
- **`ci-height`**: Height (4 bytes hex, big-endian)
- **`ci-block`**: Block hash (32 bytes hex)
- **`compute-index`**: Height + transaction ID (4 + 32 bytes hex)
- **`block-tx`**: Block hash + position (32 + 4 bytes hex)
- **`tx-occur`**: Transaction ID + block hash (32 + 32 bytes hex)
- **Static keys**: Block hash (32 bytes hex)

### Examples

```bash
# Lookup transaction by ID (data portion only)
go run main.go lookup --key-type tx --key "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"

# Lookup CI height mapping (height 100000 = 0x000186a0)
go run main.go lookup --key-type ci-height --key "000186a0"

# Lookup compute index (height 892230 = 0x000D9D46 + txid)
go run main.go lookup --key-type compute-index --key "000D9D46a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef"

# Lookup with full key including prefix (alternative approach)
go run main.go lookup --key-type compute-index --key "0d000D9D46a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef"
```

## Database Structure

The compute index keys store tweak and output information for each transaction, organized by height and transaction ID. This allows for efficient querying of transaction data within specific height ranges.

## Requirements

- Go 1.19 or later
- Access to a Blindbit Oracle pebble database
- The database must be properly initialized with compute index data
