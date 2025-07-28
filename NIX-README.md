# Blindbit Oracle Nix Flake

This Nix flake provides a complete build and development environment for the Blindbit Oracle - a Silent Payments indexing server for Bitcoin.

## Supported Platforms

- **macOS ARM64** (`aarch64-darwin`)
- **Linux x86_64** (`x86_64-linux`)  
- **Linux ARM64** (`aarch64-linux`)

## Quick Start

### Building and Running

```bash
# Build the application
nix build

# Run the application
nix run

# Enter development shell
nix develop
```

### Development Environment

The development shell includes:
- Go 1.24
- just (task runner)
- golangci-lint for code quality
- delve for debugging
- git

```bash
nix develop
```

Upon entering the development shell, you'll see available commands and next steps.

## Available Commands (via Just)

Once in the development environment, use `just` for common tasks:

### Core Commands
- `just build` - Build the application
- `just run` - Start the server with development config
- `just test` - Run tests

### Development Commands  
- `just init` - Setup default config file (run this first)
- `just show-config` - Display current configuration
- `just check` - Run tests and linting

### Additional Commands
- `just test-verbose` - Run tests with verbose output
- `just test-coverage` - Run tests with coverage reporting
- `just lint` - Run golangci-lint
- `just fmt` - Format Go code
- `just clean` - Clean build artifacts
- `just clean-config` - Remove config file
- `just help` - Show all available commands

## Configuration

### Initial Setup

```bash
# Enter development environment
nix develop

# Initialize configuration (creates ~/.blindbit-oracle/blindbit.toml)
just init

# View current configuration
just show-config
```

### Configuration Options

The configuration file (`~/.blindbit-oracle/blindbit.toml`) supports these options:

```toml
# Server host and port
host = "127.0.0.1:8000"  # Use "0.0.0.0:8000" for external access

# Bitcoin network
chain = "signet"  # Options: "main", "testnet", "signet", "regtest"

# Bitcoin RPC connection
rpc_endpoint = "http://127.0.0.1:18443"  # Adjust port for your network
rpc_user = "your-rpc-user"               # Required unless using cookie_path
rpc_pass = "your-rpc-password"           # Required unless using cookie_path
cookie_path = ""                         # Alternative to user/pass auth

# Sync settings
sync_start_height = 1                    # Starting block height (>= 1)
max_parallel_tweak_computations = 4      # Should match CPU cores
max_parallel_requests = 4                # Limited by Bitcoin RPC capacity

# Index configuration
tweaks_only = 0                          # Generate only tweaks (0 or 1)
tweaks_full_basic = 1                    # Basic tweak index (0 or 1)
tweaks_full_with_dust_filter = 0         # Full index with dust filtering (0 or 1)
tweaks_cut_through_with_dust_filter = 0  # Cut-through with dust filtering (0 or 1)
```

### Network-Specific RPC Endpoints

- **mainnet**: `http://127.0.0.1:8332`
- **testnet**: `http://127.0.0.1:18332`
- **signet**: `http://127.0.0.1:38332`
- **regtest**: `http://127.0.0.1:18443`

## Development Workflow

```bash
# 1. Enter development environment
nix develop

# 2. Initialize configuration
just init

# 3. Edit configuration as needed
# Edit ~/.blindbit-oracle/blindbit.toml

# 4. Build and test
just dev

# 5. Run the server
just run
```

## Troubleshooting

### Common Issues

1. **Configuration not found:**
   ```bash
   just init  # Creates ~/.blindbit-oracle/blindbit.toml
   ```

2. **Connection refused to Bitcoin RPC:**
   - Check `rpc_endpoint` matches your Bitcoin node
   - Verify `rpc_user` and `rpc_pass` or `cookie_path`
   - Ensure Bitcoin RPC is enabled (`server=1` in bitcoin.conf)

3. **Build issues:**
   ```bash
   just clean   # Clean build artifacts
   just build   # Rebuild
   ```

### Getting Help

```bash
just help        # Show available commands
just --list      # List all recipes
nix develop      # Enter shell with helpful startup message
```

## Contributing

When modifying the project:

1. Enter development environment: `nix develop`
2. Make changes
3. Run quality checks: `just check`
4. Test your changes: `just test`
5. Build: `just build`

The Nix flake automatically manages Go dependencies and provides a consistent development environment across all supported platforms.
