# BlindBit Oracle CLI

The BlindBit Oracle now uses a Cobra-based CLI that provides granular control over different features.

## Prerequisites

First, add Cobra to your dependencies:

```bash
go get github.com/spf13/cobra
```

## Available Commands

### `static-indexes` - Build Static Indexes
Builds static indexes for all blocks in the database without starting continuous scanning or servers.

```bash
./blindbit-oracle static-indexes [flags]
```

**What it does:**
- Processes all blocks from the first block to the current tip
- Creates static indexes for tweaks and outputs
- Exits after completion

**Use case:** When you want to rebuild indexes without running the full service.

### `sync` - Initial Blockchain Sync
Performs initial blockchain sync to the current tip without starting continuous scanning or servers.

```bash
./blindbit-oracle sync [flags]
```

**What it does:**
- Syncs all blocks from the first block to the current tip
- Exits after completion
- Does not rebuild static indexes

**Use case:** When you want to sync the blockchain once without running the full service.

### `run` - Full Service
Runs the complete BlindBit Oracle service including all features.

```bash
./blindbit-oracle run [flags]
```

**What it does:**
- Initial blockchain sync
- Static index building
- Continuous scanning for new blocks
- HTTP API server
- gRPC server (if configured)

**Use case:** Production deployment or when you want the full service running.

## Global Flags

All commands support these global flags:

- `--datadir <path>`: Set the base directory for blindbit oracle (default: ~/.blindbit-oracle)
- `--config <path>`: Path to config file (default: datadir/blindbit.toml)
- `--version`: Show version information

## Examples

```bash
# Build static indexes only
./blindbit-oracle static-indexes

# Sync blockchain once
./blindbit-oracle sync

# Run full service
./blindbit-oracle run

# Use custom data directory
./blindbit-oracle --datadir /custom/path run

# Use custom config file
./blindbit-oracle --config /path/to/config.toml run
```

## Migration from Old Version

The old single-command behavior is now equivalent to:

```bash
./blindbit-oracle run
```

## Help

Get help for any command:

```bash
./blindbit-oracle --help
./blindbit-oracle static-indexes --help
./blindbit-oracle sync --help
./blindbit-oracle run --help
```
