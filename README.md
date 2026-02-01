# BlindBit Oracle

A GO implementation for a BIP0352 Silent Payments Indexing Server.
This backend was focused on serving the BlindBit light client suite with tweak data and other simplified data to spend and receive.
The produced index matches [other implementations](https://github.com/bitcoin/bitcoin/pull/28241#issuecomment-2079270744).

> **Note:** This is version 2.0 - a major rewrite and refactoring of BlindBit Oracle. The database logic has been completely rewritten, it uses a different Bitcoin Core backend version, and all API endpoints have changed. If you need the previous version, please use the [`v1` branch](../../tree/v1).

## What's New in v2

This version represents a complete rewrite and refactoring of BlindBit Oracle with significant improvements:

- **Database Backend Migration**: Migrated from LevelDB to PebbleDB for improved performance and reliability
- **Bitcoin Core Integration**: Switched from RPC to REST API for Bitcoin Core communication (requires Core v30+)
- **API Updates**: HTTP and gRPC endpoints have been updated and restructured
- **Performance Optimizations**: Enhanced indexing algorithms and database operations
- **CLI Improvements**: Better command structure with granular control over sync, indexing, and server operations

**Breaking Changes**: All API endpoints from v1 are incompatible with v2. For detailed API documentation, see [`internal/server/README.md`](internal/server/README.md).

## Prerequisites

- Go 1.24.1 or later
- Bitcoin Core full node with REST API access
- **Required:** Bitcoin Core v30 or later (must include [PR #32540](https://github.com/bitcoin/bitcoin/pull/32540) for `/rest/spenttxouts/:blockhash.bin` endpoint support)
- Unpruned Bitcoin Core node (required for accessing spent transaction outputs)
- Sufficient disk space for index storage (varies by chain and sync height)

## Build

1. Clone this repository
2. Build the binary (dependencies will be downloaded automatically):

   ```bash
   go build -o blindbit-oracle ./cmd/blindbit-oracle
   ```

## Run

The BlindBit Oracle uses a Cobra-based CLI with granular control over different features.

### Available Commands

#### `sync` - Initial Blockchain Sync

Performs initial blockchain sync to the current tip without starting continuous scanning or servers.

```bash
./blindbit-oracle sync
```

## Feature Modes

The server supports different storage strategies configured via `blindbit.toml`.

### Storage Flags

These flags control **how tweaks are stored**:

| Flag | `/tweak-index` | `/tweaks` | Storage |
|------|----------------|-----------|---------|
| `tweaks_full_basic=1` | works (no dust) | empty | ~1.7GB |
| `tweaks_full_with_dust_filter=1` | works (with dust) | empty | ~1.7GB + dust data |
| `tweaks_cut_through_with_dust_filter=1` | empty | works (with dust) | ~2.8GB (prunable) |

**At least one storage flag must be enabled**, otherwise tweaks are computed but discarded (the server will log a warning).

### The `tweaks_only` Flag

The `tweaks_only` flag controls whether to **skip UTXO processing** (filters, spent index, etc.), NOT whether to store tweaks.

| Config | Behavior |
|--------|----------|
| `tweaks_only=0` | Full processing: tweaks + UTXOs + filters |
| `tweaks_only=1` | Skip UTXO processing, only handle tweaks |

**Important:** `tweaks_only=1` must be combined with a storage flag (`tweaks_full_basic` or `tweaks_full_with_dust_filter`) to be useful. On its own, tweaks are computed and discarded.

**Note:** `tweaks_only=1` cannot be combined with `tweaks_cut_through_with_dust_filter=1` (cut-through requires UTXO tracking to prune spent outputs).

### Example Configurations

```toml
# Full server with block-level index (default)
tweaks_only = 0
tweaks_full_basic = 1

# Tweak-only server (no UTXO tracking, saves storage/processing)
tweaks_only = 1
tweaks_full_basic = 1

# Full server with dust filtering on block index
tweaks_only = 0
tweaks_full_with_dust_filter = 1

# Full server with cut-through (requires UTXO tracking)
tweaks_only = 0
tweaks_cut_through_with_dust_filter = 1
```

### Client Discovery

Clients should call `/info` to discover which features are enabled and use the appropriate endpoint:

```json
{
  "network": "signet",
  "height": 834761,
  "tweaks_only": false,
  "tweaks_full_basic": true,
  "tweaks_full_with_dust_filter": false,
  "tweaks_cut_through_with_dust_filter": false
}
```

- If `tweaks_full_basic` or `tweaks_full_with_dust_filter`: use `/tweak-index`
- If `tweaks_cut_through_with_dust_filter`: use `/tweaks`

## DiskUsage

```bash
./blindbit-oracle run
```

### Global Flags

All commands support these global flags:

- `--datadir <path>`: Set the base directory for blindbit oracle (default: `~/.blindbit-oracle`)
- `--config <path>`: Path to config file (default: `datadir/blindbit.toml`)

### Configuration

Create a config file `blindbit.toml` in your data directory. An example [blindbit.example.toml](blindbit.example.toml) is provided.

**v2 Configuration Changes:**

- **Backend change**: Switched from Bitcoin Core RPC to REST API (`core_rest_endpoint` instead of `rpc_endpoint`, `rpc_user`, `rpc_pass`)
- **Server configuration**: Separate `http_host` and `grpc_host` instead of single `host` parameter
- **New options**: Added `log_level` and `max_cpu_cores` configuration parameters
- **Database backend**: Migrated from LevelDB to PebbleDB for improved performance
- **Legacy flags**: Existing indexing options now marked as legacy with minimal impact

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

## API Documentation

The BlindBit Oracle provides HTTP and gRPC APIs for accessing silent payment data. For detailed API documentation including endpoint specifications, request/response formats, and examples, see:

- **HTTP API**: [`internal/server/README.md`](internal/server/README.md)
- **gRPC API**: See the protobuf definitions and generated service endpoints

### Available HTTP Endpoints

- `GET /tweaks/:blockheight` - Simple list of tweaks (33-byte public keys)
- `GET /utxos/:blockheight` - UTXO information for blocks  
- `GET /spent-outputs/:blockheight` - Shortened spent output information
- `GET /compute-index/:blockheight` - Compact transaction index with tweak mappings
- `GET /full-block/:blockheight` - Complete block data with all transaction details

### Help

Get help for any command:

```bash
./blindbit-oracle --help
./blindbit-oracle sync --help
./blindbit-oracle server-only --help
./blindbit-oracle run --help
 ```
