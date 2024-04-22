# BlindBit Backend

A GO implementation for a BIP0352 Silent Payments Indexing Server.
This backend was focused on serving the BlindBit mobile app with tweak data and other simplified data to spend and
receive.

## Setup

The installation process is still very manual. Will be improved based on feedback and new findings.

### Prerequisites

- You need go 1.18 installed

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

## Requirements

- RPC access to a bitcoin full node
    - unpruned because we need the prevouts for every transaction in the block with a taproot output
    - Note: Indexing will take longer if the rpc calls take longer;
      You might also want to allow more rpc workers on your node to speed things up.
- Processing a block takes ~100ms-300ms
- Disk space

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

## Todos

- [ ] Convert env vars into command line args
- [ ] Include [gobip352 module](https://github.com/setavenger/gobip352)
- [ ] Refactor a bunch of stuff to use byte arrays or slices instead of strings for internal uses
    - Could potentially reduce the serialisation overhead
    - In combination with proto buffs we might not even have to convert for serving the API
- [ ] Introduce Proto buffs
- [ ] Clean up code (bytes.Equal, parity on big.int with .Bit(), etc.)
- [ ] Update to new test vectors
- [ ] Write operation tests to ensure data integrity
- [ ] Benchmark btcec vs libsecp C library wrapper/binding
    - probably need to create my own wrapper
    - https://github.com/renproject/secp256k1
    - https://github.com/ethereum/go-ethereum/tree/master/crypto/secp256k1/libsecp256k1 (~8 years without update)
- [ ] Periodically recompute filters
    - One could implement a periodic re-computation every 144 blocks of filters with the current UTXO set
- [ ] Investigate whether we should change the compound keys to use the height instead of the hash. As keys are sorted
  this could potentially give a performance boost due to better order across blocks.
    - ON_HOLD: Not reorg resistant unless some extra work and checks are made
- [ ] Document EVERYTHING: especially serialization patterns to easily look them up later.
    - Serialisation
    - tweak computation methods
    - ...
- [x] Redo the storage system. After syncing approximately 5,500 blocks, the estimated storage at 100,000 blocks for
  tweaks alone will be somewhere around 40Gb. Additionally, performance is getting worse.
    - Done: Switched to LevelDB see here
      for [current numbers](https://github.com/setavenger/BIP0352-light-client-specification)
- [x] Investigate whether RPC parallel calls can speed up syncing. Caution: currently the flow is synchronous and hence
  there is less complexity. Making parallel calls will change that.
    - note: This was mainly limited by a slow home node. First tests a more performant node show that this is not as big
      as a problem. Also using parallel calls on a weak node just increases the latency for every individual call
      reducing most of the gains from parallel calls.
- [ ] Include redundancy for when RPC calls are failing (probably due to networking issues in a testing home
  environment).
- [ ] Review all duplicate key error exemptions and raise to error/warn from debug.
- [ ] Remove unnecessary panics.
- [ ] Future non priority: move tweak computation code into another repo
- [ ] Convert hardcoded serialisation assertions into constants (?)
- [ ] Use x-only 32 byte public keys instead of scriptPubKey
