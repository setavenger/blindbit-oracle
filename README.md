# BlindBit Backend

A GO implementation for a BIP0352 Silent Payments Indexing Server.
This backend was focused on serving the BlindBit light clients with tweak data and other simplified data to spend and
receive. The produced index matches that
of [other implementations](https://github.com/bitcoin/bitcoin/pull/28241#issuecomment-2079270744).

## Setup

The installation process is still very manual. Will be improved based on feedback and new findings.

### Requirements

- RPC access to a bitcoin full node
    - unpruned because we need the prevouts for every transaction in the block with a taproot output
    - Note: Indexing will take longer if the rpc calls take longer;
      You might also want to allow more rpc workers on your node to speed things up.
- Processing a block takes ~100ms-300ms
- Disk space (~10Gb)
- go 1.18 installed

### Build

1. Clone this repo
2. Navigate into the repo and build `go build -o <path/to/new/binary/file> ./src`

### Run

Set the necessary ENV variables. An example is shown below.
All those should be set but the exact numbers depend on the machine the program is run on.

```text
export BASE_DIRECTORY="./test" 
export MAX_PARALLEL_REQUESTS=4  (depends on max-rpc-workers of the underlying full node)
export RPC_ENDPOINT="http://127.0.0.1:18443" (defaults to http://127.0.0.1:8332)
export RPC_PASS="your-rpc-password"
export RPC_USER="your-rpc-user"
export SYNC_START_HEIGHT=1 (has to be >= 1)

export MAX_PARALLEL_TWEAK_COMPUTATIONS=4 (the default for this is 1, but should be set to a higher value to increase performance, one should set this in accordance to how many cores one wants to use)

[optional]
export TWEAKS_ONLY=1 (default: 0; 1 to activate | will only generate tweaks)
```

Once the ENV variables are set you can just run the binary.

## Known Errors

- block 727506 no tweaks but still one utxo listed (this should not happen)
    - REASON: UTXOs are currently blindly added based on being taproot. There is no check whether the inputs are
      eligible. Will be fixed asap.
- cleanup has an error on block 712,517 as per
  this [issue](https://github.com/setavenger/BlindBit-Backend/issues/2#issuecomment-2069827679). Needs fixing asap.
    - program can only be run in tweak only mode for the time being

## Todos

- [ ] Add flags to control setup
  - reindex
  - headers only
  - tweaks only
  - move most env controls to config file or cli flags/args
- [ ] Include [gobip352 module](https://github.com/setavenger/gobip352)
- [ ] Refactor a bunch of stuff to use byte arrays or slices instead of strings for internal uses
    - Could potentially reduce the serialisation overhead
    - In combination with proto buffs we might not even have to convert for serving the API
- [ ] Introduce Proto buffs
- [ ] Clean up code (bytes.Equal, parity on big.int with .Bit(), etc.)
- [ ] Update to new test vectors
- [ ] Write operation tests to ensure data integrity
- [ ] Periodically recompute filters
    - One could implement a periodic re-computation every 144 blocks of filters with the current UTXO set
- [ ] Document EVERYTHING: especially serialization patterns to easily look them up later.
    - Serialisation
    - tweak computation methods
- [ ] Include redundancy for when RPC calls are failing (probably due to networking issues in a testing home
  environment).
- [ ] Review all duplicate key error exemptions and raise to error/warn from debug.
- [ ] Remove unnecessary panics.
- [ ] Future non priority: move tweak computation code into another repo
- [ ] Convert hardcoded serialisation assertions into constants (?)
- [ ] Use x-only 32 byte public keys instead of scriptPubKey

### Low Priority
- [ ] Index the next couple blocks in mempool
  - Every 1-3 minute or so?

## Endpoints

```text
GET("/block-height")  // returns the height of the indexing server
GET("/tweaks/:blockheight")  // returns tweak data (cut-through)
GET("/tweak-index/:blockheight")  // returns the full tweak index (no cut-through)
GET("/filter/:blockheight") // returns a custom taproot only filter (the underlying data is subject to change; changing scriptPubKey to x-only pubKey) 
GET("/utxos/:blockheight")  // UTXO data for that block (cut down to the essentials needed to spend)
```

## DiskUsage

This is roughly the space needed. Some changes were made to the indexing server but overall it should still be in this
range.

```text
  709632 -> 834761
  217M	./filters
  2.7G	./utxos
  16M	./headers-inv
  12M	./headers
  2.8G	./tweaks        33,679,543 tweaks
  1.7G	./tweak-index   54,737,510 tweaks
  7.4G	.
 ```