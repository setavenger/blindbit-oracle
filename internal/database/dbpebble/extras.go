package dbpebble

/*


0x01 block_txs     key = [01][32 blockHash][4 posBE]                   val = [32 txid]
0x02 transactions  key = [02][32 txid]                                 val = [33 tweak]    // only if tweak exists
0x03 outputs       key = [03][32 txid][4 voutBE]                       val = [8 amountLE][32 pubkey]  // x-only
0x04 spends        key = [04][32 prevTxid][4 prevVoutBE][32 blockHash] val = [32 spendPubkey] or []   // optional
0x05 ci:height     key = [05][4 heightBE]                              val = [32 blockHash]
0x06 ci:block      key = [06][32 blockHash]                            val = [4 heightBE]
0x07 tx-occurs     key = [07][32 txid][32 blockHash]                   val = []             // keys-only, optional
// (optional) 0x08 outv: ranking by amount per txid if you want faster dust filters later


*/
