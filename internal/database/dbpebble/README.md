# Pebble Database Schema

## Size Constants
```go
SizeHash   = 32  // Bitcoin block/tx hash
SizeTxid   = 32  // Transaction ID  
SizeHeight = 4   // Block height
SizePos    = 4   // Transaction position in block
SizeVout   = 4   // Output index
SizeTweak  = 33  // Taproot tweak
SizeAmt    = 8   // Amount (satoshi)
SizePubKey = 32  // X-only public key
```

## Database Schema

### Chain Index
| Prefix | Key Structure | Value | Description |
|--------|---------------|-------|-------------|
| `0x05` | `[0x05][height:4]` | `[blockhash:32]` | Height → Block Hash |
| `0x06` | `[0x06][blockhash:32]` | `[height:4]` | Block Hash → Height |

### Transactions
| Prefix | Key Structure | Value | Description |
|--------|---------------|-------|-------------|
| `0x01` | `[0x01][blockhash:32][position:4]` | `[txid:32]` | Block + Position → Transaction ID |
| `0x02` | `[0x02][txid:32]` | `[tweak:33]` or `nil` | Transaction → Tweak |
| `0x07` | `[0x07][txid:32][blockhash:32]` | `nil` (keys-only) | Transaction → Block Occurrence |

### Outputs
| Prefix | Key Structure | Value | Description |
|--------|---------------|-------|-------------|
| `0x03` | `[0x03][txid:32][vout:4]` | `[amount:8][pubkey:32]` | Transaction Output |

### Spending
| Prefix | Key Structure | Value | Description |
|--------|---------------|-------|-------------|
| `0x04` | `[0x04][prev_txid:32][prev_vout:4][blockhash:32]` | `[spend_pubkey:32]` or `nil` | Spent Output |

### Static Indexes
| Prefix | Key Structure | Value | Description |
|--------|---------------|-------|-------------|
| `0x08` | `[0x08][blockhash:32]` | `[tweak1:33][tweak2:33]...[tweakN:33]` | Static Tweaks |
| `0x09` | `[0x09][blockhash:32]` | `[serialized_outputs...]` | Static UTXOs |

### Filters
| Prefix | Key Structure | Value | Description |
|--------|---------------|-------|-------------|
| `0x0A` | `[0x0A][blockhash:32]` | `[gcs_filter_bytes]` | Taproot Pubkey Filter |
| `0x0B` | `[0x0B][blockhash:32]` | `[gcs_filter_bytes]` | Taproot Unspent Filter |
| `0x0C` | `[0x0C][blockhash:32]` | `[gcs_filter_bytes]` | Taproot Spent Filter |

### Accelerators
| Prefix | Key Structure | Value | Description |
|--------|---------------|-------|-------------|
| `0x0D` | `[0x0D][height:4][txid:32]` | `[serialized_compute_data]` | Compute Index |
| `0x0E` | `[0x0E][blockhash:32]` | `[pubkey_prefix1:8][pubkey_prefix2:8]...[pubkey_prefixN:8]` | Spent Outputs Short |
| `0x0F` | `[0x0F][blockhash:32][txid:32]` | `[outpoint1:36][outpoint2:36]...[outpointN:36]` | Txid → Outpoints |

## Value Encoding Details

### Output Values (`0x03`)
- **Amount**: Little-endian 8-byte uint64
- **Pubkey**: 32-byte x-only public key

### Tweak Values (`0x02`)
- **Tweak**: 33-byte Taproot tweak (or nil if no tweak)

### Spend Values (`0x04`)
- **Spend Pubkey**: 32-byte x-only public key (or nil for keys-only)

### Outpoint Values (`0x0F`)
- **Outpoint**: 36 bytes = `[txid:32][vout:4]` (big-endian vout)

### Spent Outputs Short (`0x0E`)
- **Pubkey Prefix**: First 8 bytes of x-only public keys

## Endianness
- **Big-endian**: Heights, positions, vouts (for key ordering)
- **Little-endian**: Amounts (for value encoding)

**Note**: Endianness observations may not always be correct. Bitcoin Core typically provides txids/blockhashes in little-endian format, which are stored as-is without conversion. The actual endianness depends on how data is received from Bitcoin Core and whether any conversion is performed during storage.

## Serialization Specifications

### Static Tweaks (`0x08`)
```
Value: [tweak1:33][tweak2:33]...[tweakN:33]
```
- Concatenated 33-byte tweaks
- No length prefix or count
- Fixed 33-byte per tweak

### Static UTXOs (`0x09`)
```
Value: [output1:76][output2:76]...[outputN:76]
```
- Each output is exactly 76 bytes:
  - `[txid:32][vout:4][amount:8][pubkey:32]`
- All fields little-endian except txid/pubkey (raw bytes)
- No length prefix or count

### Compute Index (`0x0D`)
```
Value: [tweak:33][output1:8][output2:8]...[outputN:8]
```
- **Tweak**: 33-byte Taproot tweak
- **Outputs**: Array of 8-byte output prefixes (first 8 bytes of x-only pubkeys)
- No length prefix for outputs array

### Spent Outputs Short (`0x0E`)
```
Value: [prefix1:8][prefix2:8]...[prefixN:8]
```
- Array of 8-byte prefixes (first 8 bytes of x-only pubkeys)
- No length prefix or count
- Empty array stored as `[]byte{}` for blocks with no spent outputs

### Txid Outpoints (`0x0F`)
```
Value: [outpoint1:36][outpoint2:36]...[outpointN:36]
```
- Each outpoint is exactly 36 bytes:
  - `[txid:32][vout:4]` (big-endian vout)
- No length prefix or count
- Empty array stored as `[]byte{}` for transactions with no outputs
