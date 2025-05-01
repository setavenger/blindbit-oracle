# TX-Analyzer
A simple way to compute the tweak of any transaction.

## Build

```bash
go build ./cmd/tx-analyzer -o tx-analyzer
```

```bash
  -blockhash string (optional)
        blockhash might be needed if txindex is not enabled on the node
  -rpc-host string
        the hostname (including port) of the bitcoin core node
  -rpc-pass string
        the nodes rpc password
  -rpc-user string
        the nodes rpc user
  -txid string
        give tx hex
```
