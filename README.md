# BlindBit Oracle

A GO implementation for a BIP0352 Silent Payments Indexing Server.
This backend was focused on serving the BlindBit light client suite with tweak data and other simplified data to spend
and receive. The produced index
matches [other implementations](https://github.com/bitcoin/bitcoin/pull/28241#issuecomment-2079270744).

## Setup

The installation process is still very manual. Will be improved based on feedback and new findings. It is advised to look at the example [blindbit.toml](blindbit.example.toml). As new config options appear they will be listed and explained there.

### Requirements

- RPC access to a bitcoin full node
    - unpruned because we need the prevouts for every transaction in the block with a taproot output
    - Note: Indexing will take longer if the rpc calls take longer;
      You might also want to allow more rpc workers on your node to speed things up.
- Processing a block takes ~100ms-300ms
- Disk space (~10Gb)
- go 1.20 installed

### Build

1. Clone this repo
2. Navigate into the repo and build `go build -o <path/to/new/binary/file> ./src`

### Run

Create a config file `blindbit.toml` in your data directory to run.
An example [blindbit.toml](./blindbit.example.toml) is provided here.
The settings in regard to parallelization have to be made in accordance to the cores on the Full node and host machine.

Once the data directory is set up you can run it as following.

```console
$ <path/to/new/binary/file> --datadir <path/to/datadir/with/blindbit.toml>
```

Note that the program will automatically default `--datadir` to `~/.blindbit-oracle` if not set.
You still have to set up a config file in any case as the rpc users can't and **should** not be defaulted.

You can now also decide which index you want to run. This setting can be set in the config file (blindbit.toml).

## Known Errors

No known issues.

## Todos

- [ ] Add flags to control setup
    - [ ] reindex
    - [ ] headers only
    - [x] tweaks only
    - [x] move most env controls to config file or cli flags/args
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
- [ ] Convert hardcoded serialisation assertions into constants (?)
- [x] Use x-only 32 byte public keys instead of scriptPubKey
- [ ] Don't create all DBs by default, only those which are needed and activated
- [ ] Check import paths (SilentPaymentBackend/.../...)

### Low Priority

- [ ] Index the next couple blocks in mempool
    - Every 1-3 minute or so?

## Endpoints

```text
GET("/block-height")  // returns the height of the indexing server
GET("/tweaks/:blockheight?dustLimit=<sat_amount>")  // returns tweak data (cut-through); optional parameter dustLimit can be omitted; filtering happens per request, so virtually any amount can be specified
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
