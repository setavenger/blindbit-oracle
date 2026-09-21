# gRPC API Specification — BlindBit Oracle

The BlindBit Oracle exposes a single gRPC service, `OracleService`, defined in
[`blindbit-lib/proto/oracle_service.proto`](https://github.com/setavenger/blindbit-rs/blob/master/blindbit-lib/proto/oracle_service.proto)
with all message types in
[`blindbit-lib/proto/indexing_server.proto`](https://github.com/setavenger/blindbit-rs/blob/master/blindbit-lib/proto/indexing_server.proto).

The server is configured via `grpc_host` in `blindbit.toml`.  If `grpc_host`
is not set the gRPC server is not started.

---

## Conventions

| Convention | Detail |
|---|---|
| Package | `blindbit.oracle.v1` |
| Block hashes | 32 bytes, **little-endian** (reversed from raw Bitcoin bytes) |
| Transaction IDs | 32 bytes, **little-endian** (reversed from raw Bitcoin bytes) |
| Tweaks | 33-byte compressed public key (BIP-352 summed input tweak) |
| Shortened pubkeys | First 8 bytes of an x-only taproot output pubkey |
| Outpoints | 36 bytes = 32-byte txid (little-endian) + 4-byte vout (little-endian uint32) |
| Amounts | uint64, satoshis |
| `bytes` fields | Flat (contiguous) byte arrays; divide by the stated entry size to get the count |

gRPC reflection is enabled on the server, so `grpc_cli` and similar tools can
introspect the service at runtime.

---

## Discovery

### `GetInfo`

Returns server metadata.  **Call this first** to learn which storage modes
are active before issuing data requests.

```
rpc GetInfo(google.protobuf.Empty) returns (InfoResponse)
```

**Response — `InfoResponse`:**

| Field | Type | Description |
|---|---|---|
| `network` | string | Bitcoin network name (`"mainnet"`, `"signet"`, `"testnet"`) |
| `height` | uint64 | Chain-tip height that has been fully indexed |
| `tweaks_only` | bool | UTXO processing is disabled; spent/full-block data will be empty |
| `tweaks_full_basic` | bool | Full per-transaction tweaks stored without dust filter |
| `tweaks_full_with_dust_filter` | bool | Full per-transaction tweaks stored with dust filter |
| `tweaks_cut_through_with_dust_filter` | bool | Cut-through tweaks stored (spent outputs pruned) with dust filter |

---

### `GetBestBlockHeight`

```
rpc GetBestBlockHeight(google.protobuf.Empty) returns (BlockHeightResponse)
```

Returns only the current chain-tip height (`block_height: uint64`).

---

### `GetBlockHashByHeight`

```
rpc GetBlockHashByHeight(BlockHeightRequest) returns (BlockHashResponse)
```

**Request:** `block_height: uint64`

**Response:** `block_hash: bytes` — 32 bytes, little-endian.

---

## Block data — streaming

Both streaming RPCs accept a `RangedBlockHeightRequestFiltered` request and
emit one response message per block, in ascending height order.  The stream
ends after the last requested height has been sent.

**`RangedBlockHeightRequestFiltered` fields:**

| Field | Type | Description |
|---|---|---|
| `start` | uint64 | First block height to include (inclusive) |
| `end` | uint64 | Last block height to include (inclusive) |
| `dustlimit` | uint64 | Sats. Drop transactions with no surviving output at or above this. `StreamComputeIndex` only, *reserved* on `StreamBlockScanDataShort` |
| `cut_through` | bool | Drop transactions whose outputs are all spent at the index tip. `StreamComputeIndex` only, *reserved* on `StreamBlockScanDataShort` |

Both filters apply per transaction, never per output. A transaction is kept
when at least one of its outputs is unspent at the pinned tip and at or above
`dustlimit`, and a kept transaction carries all of its output prefixes.
Removing single outputs would stop a scanner at the first `k` it cannot match
and hide every later output of that transaction. The tip is read once at the
start of the stream. `dustlimit: 0` with `cut_through: false` returns the
unfiltered index byte for byte.

---

### `StreamBlockScanDataShort` *(recommended)*

The primary endpoint for incremental wallet scanning.  For each block it
sends both the compute index and the spent-output index in one message,
eliminating the need for a second call.

```
rpc StreamBlockScanDataShort(RangedBlockHeightRequestFiltered)
    returns (stream BlockScanDataShortResponse)
```

**Response per block — `BlockScanDataShortResponse`:**

| Field | Type | Description |
|---|---|---|
| `block_identifier` | BlockIdentifier | Block hash + height |
| `comp_index` | repeated ComputeIndexTxItem | See [ComputeIndexTxItem](#computeindextxitem) below |
| `spent_outputs` | bytes | Flat array of 8-byte shortened pubkeys for every spent output; `len / 8` = count |

---

### `StreamComputeIndex`

Use when you need tweak/output data but not the spent-output index.

```
rpc StreamComputeIndex(RangedBlockHeightRequestFiltered)
    returns (stream ComputeIndexResponse)
```

**Response per block — `ComputeIndexResponse`:**

| Field | Type | Description |
|---|---|---|
| `block_identifier` | BlockIdentifier | Block hash + height |
| `index` | repeated ComputeIndexTxItem | See [ComputeIndexTxItem](#computeindextxitem) below |

#### ComputeIndexTxItem

One entry per taproot-eligible transaction in the block.

| Field | Type | Description |
|---|---|---|
| `txid` | bytes | 32-byte transaction ID, little-endian |
| `tweak` | bytes | 33-byte BIP-352 input tweak |
| `outputs_short` | bytes | Flat array of 8-byte shortened x-only pubkeys; `len / 8` = output count |

---

## Block data — single block

### `GetFullBlock`

Returns the complete data set for one block.  This endpoint is
bandwidth-heavy; use it only when the full input outpoint set is required
(e.g. building a local UTXO index).

```
rpc GetFullBlock(BlockHeightRequest) returns (FullBlockResponse)
```

**Request:** `block_height: uint64`

**Response — `FullBlockResponse`:**

| Field | Type | Description |
|---|---|---|
| `block_identifier` | BlockIdentifier | Block hash + height |
| `index` | repeated FullTxItem | One entry per taproot-eligible transaction |

#### FullTxItem

| Field | Type | Description |
|---|---|---|
| `txid` | bytes | 32-byte transaction ID, little-endian |
| `tweak` | bytes | 33-byte BIP-352 input tweak |
| `inputs` | bytes | Flat array of 36-byte outpoints (32-byte txid LE + 4-byte vout LE); `len / 36` = input count |
| `utxos` | repeated UTXOItemLight | All taproot outputs in the transaction |

#### UTXOItemLight

| Field | Type | Description |
|---|---|---|
| `vout` | uint32 | Output index within the transaction |
| `amount` | uint64 | Value in satoshis |
| `pubkey` | bytes | 32-byte x-only public key |

---

### `GetSpentOutputsShort`

Returns the flat spent-output byte array for a single block (same data as the
`spent_outputs` field in `StreamBlockScanDataShort`).

```
rpc GetSpentOutputsShort(BlockHeightRequest) returns (IndexResponse)
```

**Request:** `block_height: uint64`

**Response — `IndexResponse`:**

| Field | Type | Description |
|---|---|---|
| `block_identifier` | BlockIdentifier | Block hash + height |
| `index` | bytes | Flat array of 8-byte shortened x-only pubkeys for spent outputs; `len / 8` = count |

Returns an empty `index` when no spent outputs are recorded for the block
(e.g. height below the index start, or `tweaks_only=true`).

---

## Error codes

| gRPC status | Meaning |
|---|---|
| `NOT_FOUND` | Requested height has not been indexed yet |
| `INTERNAL` | Database or server error; check oracle logs |
| `INVALID_ARGUMENT` | Malformed request (e.g. `end < start`) |

---

## Example (Rust)

```rust
use blindbit_lib::oracle_grpc::{
    RangedBlockHeightRequestFiltered,
    oracle_service_client::OracleServiceClient,
};

let mut client = OracleServiceClient::connect("https://oracle.example.com").await?;

let mut stream = client
    .stream_block_scan_data_short(RangedBlockHeightRequestFiltered {
        start: 900_000,
        end:   900_010,
        dustlimit: 0,
        cut_through: false,
    })
    .await?
    .into_inner();

while let Some(block) = stream.message().await? {
    let id = block.block_identifier.unwrap();
    println!("height {}: {} txs, {} spent-output bytes",
        id.block_height,
        block.comp_index.len(),
        block.spent_outputs.len());
}
```

See [`blindbit-lib/examples/stream_oracle.rs`](https://github.com/setavenger/blindbit-rs/blob/master/blindbit-lib/examples/stream_oracle.rs)
for a runnable example.
