
# Server HTTP API Specification

This document describes the HTTP API endpoints for the BlindBit Oracle server.

**Deprecation:** The HTTP JSON API is **deprecated** except **`GET /info`**, which remains supported for lightweight discovery of network, chain tip height, and tweak-related feature flags. New clients should use the **gRPC** `OracleService` (see protobuf definitions in `blindbit-lib`) for data access; gRPC **`GetInfo`** exposes the same discovery fields as HTTP **`GET /info`**.

## Endpoint Types

The API provides the following endpoint types (all block-data types below are **deprecated** over HTTP):
- **Info** — Feature flags and chain tip (`GET /info` supported; gRPC `GetInfo` equivalent)
- **Tweaks** - Simple list of tweaks (33-byte public keys)
- **Outputs/UTXOs** - UTXO information for blocks
- **Spent Outputs** - Shortened spent output information
- **Compute Index** - Compact transaction index with tweak mappings
- **Full Block** - Complete block data with all transaction details

## API Endpoints

### Info (supported)

`GET /info` returns chain and feature flags (same information as gRPC `GetInfo`).

**Response format:**
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

### Tweaks (deprecated HTTP)

Returns a simple list of tweaks as 33-byte public keys. For bandwidth-constrained clients, using 64/65-byte keys might be more ideal. For mappings with transaction IDs, use the Compute Index endpoint instead.

**Response Format:**
```json
{
    "block_identifier": {
        "block_hash": "0000003223acbdef....",
        "block_height": 894012
    },
    "index": [
        "03<x-only pubkey>",
        "02<x-only pubkey>",
        "03<x-only pubkey>",
        "03<x-only pubkey>",
        "02<x-only pubkey>",
        "03<x-only pubkey>",
        "02<x-only pubkey>"
    ]
}
```

### Outputs/UTXOs (deprecated HTTP)

Returns UTXO information for a specific block.

**Response Format:**
```json
{
    "block_identifier": {
        "block_hash": "0000003223acbdef....",
        "block_height": 894012
    },
    "index": [
        {
            "txid": "deadbeef",
            "vout": 0,
            "pubkey": "<x-only pubkey>",
            "amount": 210042
        },
        {
            "txid": "deadbeef",
            "vout": 1,
            "pubkey": "<x-only pubkey>",
            "amount": 220042
        },
        {
            "txid": "beefdead",
            "vout": 3,
            "pubkey": "<x-only pubkey>",
            "amount": 310021
        }
    ]
}
```

### Spent Outputs (Shortened) (deprecated HTTP)

Returns spent output information in a compact format using the first 8 bytes of output x-only pubkeys as an array of hex strings.

**Response Format:**
```json
{
    "block_identifier": {
        "block_hash": "0000003223acbdef....",
        "block_height": 894012
    },
    "index": [
        "12345acbdef12345",
        "67890fedcba98765",
        "abcdef1234567890"
    ]
}
```
_Open question: Should we jump straight to outpoints and not do this with shortened outputs?_

### Compute Index (deprecated HTTP)

Returns a compact transaction index with tweak mappings and output information.

**Response Format:**
```json
{
    "block_identifier": {
        "block_hash": "0000003223acbdef....",
        "block_height": 894012
    },
    "index": [
        {
            "txid": "deadbeef987654",
            "tweak": "02deadbeef",
            "outputs": [
                "12345acbdef12345",
                "67890fedcba98765",
                "abcdef1234567890"
            ]
        },
        {
            "txid": "beefdead1234",
            "tweak": "02deadbeef",
            "outputs": [
                "fedcba9876543210",
                "13579bdf2468ace0"
            ]
        },
        {
            "txid": "beef987654dead",
            "tweak": "02deadbeef",
            "outputs": [
                "2468ace13579bdf0"
            ]
        }
    ]
}
```

### Full Block (deprecated HTTP)

Returns complete block data with all transaction details and spent outpoints accelerator index. This endpoint provides comprehensive information but should be used sparingly due to the large amount of data.

**Response Format:**
```json
{
    "block_identifier": {
        "block_hash": "0000003223acbdef....",
        "block_height": 894012
    },
    "index": [
        {
            "txid": "deadbeef987654",
            "tweak": "02deadbeef",
            "inputs": [
                "<36byte outpoint hex>",
                "<36byte outpoint hex>",
            ],
            "utxos": [
                {
                    "vout": 0,
                    "pubkey": "<x-only pubkey>",
                    "amount": 210042
                },
                {
                    "vout": 1,
                    "pubkey": "<x-only pubkey>",
                    "amount": 220042
                }
            ]
        },
        {
            "txid": "987654deadbeef",
            "tweak": "02beefdeadefddeefdad",
            "inputs": [
                "<36byte outpoint hex>",
                "<36byte outpoint hex>",
            ],
            "utxos": [
                {
                    "vout": 0,
                    "pubkey": "<x-only pubkey>",
                    "amount": 380042
                },
                {
                    "vout": 1,
                    "pubkey": "<x-only pubkey>",
                    "amount": 380021
                }
            ]
        }
    ]
}
```

## Data Format Notes

- **Block Hash**: 32-byte block hash represented as hex string
- **Block Height**: Unsigned 32-bit integer
- **Transaction ID**: 32-byte transaction hash represented as hex string
- **Tweak**: 33-byte compressed public key
- **Output Short**: First 8 bytes of x-only pubkey represented as hex string
- **Spent Outpoint**: 36-byte outpoint (32-byte txid + 4-byte vout) represented as hex string
- **Amount**: Transaction output amount in satoshis

## Full Block Response Details (deprecated HTTP)

The deprecated Full Block HTTP endpoint includes:

- **index**: Array of transaction items with tweaks and UTXOs
- **spent_outpoints**: Array of all outpoints (previous transaction outputs) that were spent in this block
  - Each outpoint is 36 bytes: 32-byte previous transaction ID + 4-byte previous output index
  - Transaction IDs are reversed (little-endian) for consistency with Bitcoin conventions
  - This accelerator index provides efficient access to all spent outputs without requiring individual transaction parsing