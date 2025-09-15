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

## Database Structure

The compute index keys store tweak and output information for each transaction, organized by height and transaction ID. This allows for efficient querying of transaction data within specific height ranges.

## Requirements

- Go 1.19 or later
- Access to a Blindbit Oracle pebble database
- The database must be properly initialized with compute index data
