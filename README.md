# BlindBit Backend
A GO implementation for a BIP0352 Silent Payments Indexing Server. 
This backend was focused on serving the BlindBit mobile app with tweak data. 

## Requirements
- rpc access to a bitcoin full node 
  - unpruned because we need the prevouts for every transaction in the block with a taproot output
  - Note: Indexing will take longer if the rpc calls take longer 
  - Processing a block takes ~100ms-500ms
- disk space
  - ~500mb-1GB with cut-through (@828000 about 250mb taken)
  - +4GB if you have full-indexing enabled (@828000 about 2GB taken)

## Todos

- [ ] Write operation tests to ensure data integrity
- [ ] Benchmark btcec vs libsecp C library wrapper/binding
  - https://github.com/renproject/secp256k1
  - https://github.com/ethereum/go-ethereum/tree/master/crypto/secp256k1/libsecp256k1 (~8 years without update)
- [ ] Periodically recompute the filters? 
  - One could implement a periodic re-computation every 1000(?) blocks of the old filters with the current UTXO set.
- [ ] Investigate whether we should change the compound keys to use the height instead of the hash. As keys are sorted this could potentially give a performance boost due to better order across blocks.
- [ ] Document EVERYTHING: especially serialization patterns to easily look them up later.
  - Serialisation
  - tweak computation methods
  - ...
- [x] Redo the storage system. After syncing approximately 5,500 blocks, the estimated storage at 100,000 blocks for tweaks alone will be somewhere around 40Gb. Additionally, performance is getting worse.
  - Done: Switched to LevelDB see here for [current numbers](https://github.com/setavenger/BIP0352-light-client-specification) 
- [x] Investigate whether RPC parallel calls can speed up syncing. Caution: currently the flow is synchronous and hence there is less complexity. Making parallel calls will change that.
  - note: This was mainly limited by a slow home node. First tests a more performant node show that this is not as big as a problem. Also using parallel calls on a weak node just increases the latency for every individual call reducing most of the gains from parallel calls. 
- [ ] Include redundancy for when RPC calls are failing (probably due to networking issues in a testing home environment).
- [ ] Review all duplicate key error exemptions and raise to error/warn from debug.
- [ ] Remove unnecessary panics.
- [ ] Future non priority: move tweak computation code into another repo
