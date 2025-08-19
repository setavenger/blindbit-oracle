
Database layout is inspired by libbitcoins implementation. [Link to delving bitcoin discussion about libbitcoin performance](https://delvingbitcoin.org/t/libbitcoin-for-core-people/1222)

A rough outline of what is supposed to happen below in DBML. It should be noted that in order to know where a an input was spent the block_txs need to be populated with all relevant txids i.e. transactions where either a taproot output is spent or a possible taproot output could be SP.


```dbml
Table headers {
  block_hash blob [pk]
}

Table chain_index {                     // best chain only
  block_height int  [pk, unique]
  block_hash  blob                      // fk -> headers.block_hash
}
Ref: headers.block_hash < chain_index.block_hash

Table transactions {                    // only SP-relevant txs
  txid  blob [pk]
  tweak blob
}

Table block_txs {                       // blocks → txs (ordered)
  block_hash blob                       // fk -> headers.block_hash
  position   int                        // 0 = coinbase
  txid       blob                       // fk -> transactions.txid

  indexes {
    (block_hash, position) [pk]         // leftmost-prefix covers lookups by block_hash
    (txid)
  }
}
Ref: headers.block_hash < block_txs.block_hash
Ref: transactions.txid  < block_txs.txid

Table outputs {                         // only SP/Taproot outputs you care about
  txid   blob                            // fk -> transactions.txid
  vout   int
  pubkey blob
  amount bigint

  indexes {
    (txid, vout) [pk]
    (txid)
  }
}
Ref: transactions.txid < outputs.txid

Table inputs {                          // spend events for those SP outputs
  spend_txid blob
  idx        int
  prev_txid  blob
  prev_vout  int
  pubkey blob
  // -- no spend_block_hash here (derive via block_txs + chain_index)

  indexes {
    (spend_txid, idx) [pk]
    (prev_txid, prev_vout)
  }
}

```
