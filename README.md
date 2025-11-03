# BlindBit Oracle

A GO implementation for a BIP0352 Silent Payments Indexing Server.
This backend was focused on serving the BlindBit light client suite with tweak data and other simplified data to spend and receive. 
The produced index matches [other implementations](https://github.com/bitcoin/bitcoin/pull/28241#issuecomment-2079270744).

## Prerequisites

- Go 1.24.1 or later
- RPC access to a Bitcoin Core full node
- **Important:** Your Bitcoin Core node must have [PR #32540](https://github.com/bitcoin/bitcoin/pull/32540) merged to support the `/rest/spenttxouts/:blockhash.bin` endpoint (merged in Core v30)
- Unpruned node (required for accessing spent transaction outputs)

## Build

1. Clone this repository
2. Build the binary (dependencies will be downloaded automatically):
   ```bash
   go build -o blindbit-oracle ./cmd/blindbit-oracle
   ```

## Run

The BlindBit Oracle uses a Cobra-based CLI with granular control over different features.

### Available Commands

#### `static-indexes` - Build Static Indexes
Builds static indexes for all blocks in the database without starting continuous scanning or servers.

```bash
./blindbit-oracle static-indexes
```

#### `sync` - Initial Blockchain Sync
Performs initial blockchain sync to the current tip without starting continuous scanning or servers.

```bash
./blindbit-oracle sync
```

#### `run` - Full Service
Runs the complete BlindBit Oracle service including all features (initial sync, index building, continuous scanning, HTTP API server, and gRPC server if configured).

```bash
./blindbit-oracle run
```

### Global Flags

All commands support these global flags:

- `--datadir <path>`: Set the base directory for blindbit oracle (default: `~/.blindbit-oracle`)
- `--config <path>`: Path to config file (default: `datadir/blindbit.toml`)

### Configuration

Create a config file `blindbit.toml` in your data directory. An example [blindbit.example.toml](blindbit.example.toml) is provided.

### Examples

```bash
# Sync blockchain once
# Only syncs the indexer from start height to chain tip.
# Builds the necessary indexes and exits. 
./blindbit-oracle sync

# Only run serve data without syncing
# Open the HTTP and optionally the gRPC server (if grpc host is defined in blindbit.toml).
./blindbit-oracle server-only --help

# Run full service
# Combination of sync and server-only. 
# Syncs to chain tip, opens the servers, and after initial sync automatiaclly indexes new blocks.
./blindbit-oracle run

# Use custom data directory
./blindbit-oracle --datadir /custom/path run

# Use custom config file
./blindbit-oracle --config /path/to/config.toml run
```

### Help

Get help for any command:

```bash
./blindbit-oracle --help
./blindbit-oracle sync --help
./blindbit-oracle server-only --help
./blindbit-oracle run --help
 ```
