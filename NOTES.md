# Notes

This file is to keep track of changes made over time and to have reference points for the implementation.

## Database Efficiency

### Overview

What is the underlying problem that we want to solve?
The light client needs to easily receive the necessary information to spend its UTXOs.

The current process:

1. Request tweak data and compute potential pubKeys
2. Compare computed PubKeys against filter
3. If no match is found: go to 1
4. Request Light UTXO and find the match (Considering the parameters of the filter there is a very low chance that there
   won't be a match)
5. Add UTXO to Wallet

The critical data are the tweaks, as this is new data that is not yet computed by Bitcoin Core or any other software.
Also, taproot-only filters per block are not yet used in any implementation that I'm aware of.
But taproot-only filters are not taking up too much space as there is only one per block.   
UTXOs can technically be fetched from other software via abstraction.
Hence, we need to optimise, but it's not critical to build new infrastructure for that.
Initial Idea was to keep the UTXOs in a "light" format on hand to serve this data faster.
But it's becoming apparent that another solution might be necessary.

### Schemas

#### Problem \ Tweak Data

As of `dd672ad15fe7f33b494d27cf5c1e6279d7e26d76` we are still using mongo db with a very inefficient schema.
After syncing ~5_500 blocks the estimated storage at 100_000 blocks for tweaks alone will be somewhere around 40Gb.
This does not include the index over txids which has about half the size of the data. So it could be another 20Gb.
Additionally, performance was already getting a lot worse after the first 5_500 blocks.
Currently, every row entry into the MongoDB has the fields:

- _id (wish I could drop that),
- block_hash: 32 bytes (as hex 64bytes)
- block_height: probably 4 bytes, maybe 8
- txid: 32 bytes (as hex 64bytes)
- data: 33 bytes (stored as Bytes)

In general, I'm not sure what mongoDB does under the hood as the average entry has 244 Bytes.
The outline above should not amount to 244 bytes.

#### Light UTXOs

Light UTXOs are a simple summary of a UTXO that the light client can easily use to spend.
Below I have outlined the current schema which is stored as a row.
TxidVout was added for simpler queries but automatically bloats the DB.
The fields required to properly spend an UTXO are marked.

```go

type LightUTXO struct {
   Txid         string `json:"txid" bson:"txid"` // essential to spend
   Vout         uint32 `json:"vout" bson:"vout"` // essential to spend
   Value        uint64 `json:"value" bson:"value"`               // essential to spend
   ScriptPubKey string `json:"scriptpubkey" bson:"scriptpubkey"` // essential to spend
   BlockHeight  uint32 `json:"block_height" bson:"block_height"`
   BlockHash    string `json:"block_hash" bson:"block_hash"`
   Timestamp    uint64 `json:"timestamp" bson:"timestamp"`
   TxidVout     string `json:"tx_id_vout" bson:"tx_id_vout"`
}

```

#### Spent UTXOs

After consideration this data just has to be kicked out.
Unless one is planning to basically store the entire blockchain.
It is only going to strictly grow. Therefore, this data type has to be dropped.

The original reasoning for this was to allow the light clients to track spent utxos
after tweak data and Light UTXOs were deleted. Either find another solution or drop this feature.

The consequence of dropping Spent UTXOs is that light clients will not find transactions made by them
if they are not tracked within the client or after a rescan.

### Potential solutions

~~Switching to something like LevelDB could potentially reduce the required storage by a lot.~~
LevelDB does not support nested structures hence the new approach is to use compound keys with level db. 
This might not be a solution to the storage issue but could still improve performance. This is subject to future testing.

#### Tweak Data

~~For tweak data we could drop the block_hash and block_height for every row.
I believe the structure could look something what I have outlined below.
Potentially we might have to change it in such a way that it is easier to query by block_height instead of hash.
It's probably easier for a light client to check and control with block_height than block_hashes.~~

##### Not applicable anymore
```json
{
  "block_hash_1": [
    {
      "txid": "txid_1",
      "data": "tweak_1"
    },
    {
      "txid": "txid_2",
      "data": "tweak_2"
    }
  ],
  "block_hash_2": [
    {
      "txid": "txid_3",
      "data": "tweak_3"
    },
    {
      "txid": "txid_4",
      "data": "tweak_4"
    }
  ]
}
```

##### New structure with compound keys

```json
{
   "block_hash_1:txid_1": "tweak_1",
   "block_hash_1:txid_2": "tweak_2",
   "block_hash_2:txid_3": "tweak_3",
   "block_hash_2:txid_4": "tweak_4",
   "block_hash_2:txid_5": "tweak_5"
}
```
This will not save on storage but has potential to be a lot faster for reads and writes
#### Light UTXOs

The user just needs the essential data, we can add the metadata on a per-block basis.
We can store the data as below and then add the metadata by retrieving the block_headers.

##### Not applicable anymore

```json
{
  "block_hash_1": {
    "txid_1": [
      {
        "vout": 0,
        "value": 100000,
        "scriptPubKey": "5120<x_only_pub_key>"
      },
      {
        "vout": 1,
        "value": 200000,
        "scriptPubKey": "5120<x_only_pub_key>"
      },
      {
        "vout": 10,
        "value": 500000,
        "scriptPubKey": "5120<x_only_pub_key>"
      }
    ],
    "txid_2": [
      {
        "vout": 0,
        "value": 50000,
        "scriptPubKey": "5120<x_only_pub_key>"
      },
      {
        "vout": 3,
        "value": 200000,
        "scriptPubKey": "5120<x_only_pub_key>"
      },
      {
        "vout": 6,
        "value": 500000,
        "scriptPubKey": "5120<x_only_pub_key>"
      }
    ]
  }
}
```
##### New structure with compound keys

Compound key block_hash:txid:vout: value (where <key> is either "value" or "scriptPubKey"). 
The serialisation is simple because the scriptPubKey will always have a fixed length of 34 bytes, 
we can then read in the rest as an uint. Also, all integers and uints are fixed length.
```json
{
   "block_hash_1:txid_1:0": "5120<x_only_pub_key>:10000",
   "block_hash_1:txid_1:1": "5120<x_only_pub_key>:560000",
   "block_hash_1:txid_1:10": "5120<x_only_pub_key>:360000",
   "block_hash_1:txid_2:0": "5120<x_only_pub_key>:1000000",
   "block_hash_1:txid_2:3": "5120<x_only_pub_key>:5000",
   "block_hash_1:txid_2:6": "5120<x_only_pub_key>:10000"
   
}
```