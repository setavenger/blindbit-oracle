
# Server HTTP API Specification

This document describes the HTTP API endpoints for the BlindBit Oracle server.

## Endpoint Types

The API provides the following endpoint types:
- **Tweaks** - Simple list of tweaks (33-byte public keys)
- **Outputs/UTXOs** - UTXO information for blocks
- **Spent Outputs** - Shortened spent output information
- **Compute Index** - Compact transaction index with tweak mappings
- **Full Block** - Complete block data with all transaction details

## API Endpoints

### Tweaks

Returns a simple list of tweaks as 33-byte public keys. For bandwidth-constrained clients, using 64/65-byte keys might be more ideal. For mappings with transaction IDs, use the Compute Index endpoint instead.

**Response Format:**
```json
[
    "03<x-only pubkey>",
    "02<x-only pubkey>",
    "03<x-only pubkey>",
    "03<x-only pubkey>",
    "02<x-only pubkey>",
    "03<x-only pubkey>",
    "02<x-only pubkey>"
]
```

### Outputs/UTXOs

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

### Spent Outputs (Shortened)

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

### Compute Index

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

### Full Block

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
        }
    ],
    "spent_outpoints": [
        "deadbeef9876543210deadbeef9876543210deadbeef9876543210deadbeef00000000",
        "beefdead1234567890beefdead1234567890beefdead1234567890beefdead00000001",
        "cafebabeabcdef1234cafebabeabcdef1234cafebabeabcdef1234cafebabe00000002"
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

## Full Block Response Details

The Full Block endpoint includes:

- **index**: Array of transaction items with tweaks and UTXOs
- **spent_outpoints**: Array of all outpoints (previous transaction outputs) that were spent in this block
  - Each outpoint is 36 bytes: 32-byte previous transaction ID + 4-byte previous output index
  - Transaction IDs are reversed (little-endian) for consistency with Bitcoin conventions
  - This accelerator index provides efficient access to all spent outputs without requiring individual transaction parsing