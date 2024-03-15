# Notes

This file is to keep track of changes made over time and to have reference points for the implementation.

## Tweak Computation Performance

Results from Benchmarking. Running v2 is a clear win in terms of speed for all types of blocks (many txs and few txs).
Spinning up a go routine for every tweak seems very efficient. But can it be improved? Can push the performance even a bit more? 
Next I want to try to assign a number of tweaks to a goroutine before it spins up, 
so that we don't have the overhead of a goroutine spinning up all the time. 

We variations between different benchmarking calls, 
as seen by v1 where the only one thread is used, and we still see some discrepancies.
Overall the pattern becomes clear. v4 reduces the overhead of goroutine spawning significantly but does not outperform v2 in any real way.
v2 seems to be quite optimised at this point. I'm not quite sure what one could try to boost performance except of course just utilizing more cores.
Using more cores clearly improves the performance (almost linearly in some cases). 
Parallel processing could be used for extracting [spent UTXOs](./src/core/extractutxos.go) (L:31) as well. 
This is not a priority at the moment as the processing time seems to be low.

It should be noted that these are benchmarking results when solely running the tweak computation function. 
During initial syncing/indexing there are also a lot of parallel processes for the rpc calls.   
### 12 Goroutines

```text
goos: darwin
goarch: amd64
pkg: SilentPaymentAppBackend/src/core
cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
BenchmarkTweakV4Block833000-16                66          18221455 ns/op
BenchmarkTweakV3Block833000-16                51          23240305 ns/op
BenchmarkTweakV2Block833000-16                66          18484002 ns/op
BenchmarkTweakV1Block833000-16                 8         158669041 ns/op
BenchmarkTweakV4Block833010-16                42          28893857 ns/op
BenchmarkTweakV3Block833010-16                30          37702025 ns/op
BenchmarkTweakV2Block833010-16                42          28723057 ns/op
BenchmarkTweakV1Block833010-16                 5         212243446 ns/op
BenchmarkTweakV4Block833013-16                44          28600250 ns/op
BenchmarkTweakV3Block833013-16                36          34166821 ns/op
BenchmarkTweakV2Block833013-16                42          28579243 ns/op
BenchmarkTweakV1Block833013-16                 6         190190890 ns/op
BenchmarkTweakV4Block834469-16                86          13207238 ns/op
BenchmarkTweakV3Block834469-16                91          12260387 ns/op
BenchmarkTweakV2Block834469-16                82          13145223 ns/op
BenchmarkTweakV1Block834469-16                15          75665007 ns/op
PASS
ok      SilentPaymentAppBackend/src/core        25.640s
```

### 6 Goroutines
```text
Allowed number of parallel processes (`common.MaxParallelTweakComputations`) was 6.

goos: darwin
goarch: amd64
pkg: SilentPaymentAppBackend/src/core
cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
BenchmarkTweakV4Block833000-16                34          37608796 ns/op
BenchmarkTweakV3Block833000-16                31          37644976 ns/op
BenchmarkTweakV2Block833000-16                43          35005047 ns/op
BenchmarkTweakV1Block833000-16                 8         132897864 ns/op
BenchmarkTweakV4Block833010-16                24          58622605 ns/op
BenchmarkTweakV3Block833010-16                19          62589645 ns/op
BenchmarkTweakV2Block833010-16                21          52516952 ns/op
BenchmarkTweakV1Block833010-16                 5         204381619 ns/op
BenchmarkTweakV4Block833013-16                21          54992341 ns/op
BenchmarkTweakV3Block833013-16                18          57974175 ns/op
BenchmarkTweakV2Block833013-16                28          49971872 ns/op
BenchmarkTweakV1Block833013-16                 6         184793615 ns/op
BenchmarkTweakV4Block834469-16                66          21655617 ns/op
BenchmarkTweakV3Block834469-16                67          16031086 ns/op
BenchmarkTweakV2Block834469-16                50          20486003 ns/op
BenchmarkTweakV1Block834469-16                15          68968977 ns/op
PASS
ok      SilentPaymentAppBackend/src/core        27.134s
```


### v4 Code
```go
// ComputeTweaksForBlockV4 Upgraded version of v2
func ComputeTweaksForBlockV4(block *types.Block) ([]types.Tweak, error) {
	var tweaks []types.Tweak
	var mu sync.Mutex // Mutex to protect shared resources
	var wg sync.WaitGroup

	// Create channels for transactions and results
	txChannel := make(chan types.Transaction)
	resultsChannel := make(chan types.Tweak)

	semaphore := make(chan struct{}, common.MaxParallelTweakComputations)

	// Start worker goroutines
	for i := 0; i < common.MaxParallelTweakComputations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tx := range txChannel {
				semaphore <- struct{}{} // Acquire a slot
				for _, vout := range tx.Vout {
					if vout.ScriptPubKey.Type == "witness_v1_taproot" {
						tweakPerTx, err := ComputeTweakPerTx(tx)
						if err != nil {
							common.ErrorLogger.Println(err)
							// todo errG
							break
						}
						if tweakPerTx != nil {
							resultsChannel <- types.Tweak{
								BlockHash:   block.Hash,
								BlockHeight: block.Height,
								Txid:        tx.Txid,
								Data:        *tweakPerTx,
							}
						}
						break
					}
				}
				<-semaphore // Release the slot
			}
		}()
	}
	waitForResultsChan := make(chan struct{}, 1)
	// Start a goroutine to collect results
	go func() {
		for tweak := range resultsChannel {
			mu.Lock()
			tweaks = append(tweaks, tweak)
			mu.Unlock()
		}
		waitForResultsChan <- struct{}{}
	}()

	// Feed transactions to the channel
	for _, tx := range block.Txs {
		txChannel <- tx
	}
	close(txChannel)      // Ensure to close the txChannel after sending all transactions
	wg.Wait()             // Wait for all workers to finish
	close(resultsChannel) // Close resultsChannel only after all workers are done
	<-waitForResultsChan  // wait for all results to be processed
	return tweaks, nil
}
```

### v3 Code
```go
// ComputeTweaksForBlockV3 performs worse for high tx count but faster for low tx count <800-1000 txs
func ComputeTweaksForBlockV3(block *types.Block) ([]types.Tweak, error) {
	if block.Txs == nil || len(block.Txs) == 0 {
		common.DebugLogger.Printf("%+v", block)
		common.WarningLogger.Println("Block had zero transactions")
		return []types.Tweak{}, nil
	}
	var tweaks []types.Tweak
	var muTweaks sync.Mutex // Mutex to protect tweaks slice
	var muErr sync.Mutex    // Mutex to protect error
	var wg sync.WaitGroup

	totalTxs := len(block.Txs)
	numGoroutines := common.MaxParallelTweakComputations // Number of goroutines you want to spin up
	baseBatchSize := totalTxs / numGoroutines            // Base number of transactions per goroutine
	remainder := totalTxs % numGoroutines                // Transactions that need to be distributed
	var errG error

	for i := 0; i < numGoroutines; i++ {
		start := i * baseBatchSize
		if i < remainder {
			start += i // One extra transaction for the first 'remainder' goroutines
		} else {
			start += remainder // No extra transactions for the rest
		}

		end := start + baseBatchSize
		if i < remainder {
			end++ // One extra transaction for the first 'remainder' goroutines
		}

		batch := block.Txs[start:end]

		wg.Add(1)
		go func(txBatch []types.Transaction) {
			defer wg.Done()
			var localTweaks []types.Tweak

			for _, _tx := range txBatch {
				for _, vout := range _tx.Vout {
					if vout.ScriptPubKey.Type == "witness_v1_taproot" {
						tweakPerTx, err := ComputeTweakPerTx(_tx)
						if err != nil {
							common.ErrorLogger.Println(err)
							muErr.Lock()
							if errG == nil {
								errG = err // Store the first error that occurs
							}
							muErr.Unlock()
							break
						}
						if tweakPerTx != nil {
							localTweaks = append(localTweaks, types.Tweak{
								BlockHash:   block.Hash,
								BlockHeight: block.Height,
								Txid:        _tx.Txid,
								Data:        *tweakPerTx,
							})
						}
						break // Assuming you only need one tweak per transaction
					}
				}
			}

			// Safely append to the global slice
			muTweaks.Lock()
			tweaks = append(tweaks, localTweaks...)
			muTweaks.Unlock()
		}(batch)
	}

	wg.Wait()
	return tweaks, nil
}
```

### v2 Code
```go
func ComputeTweaksForBlockV2(block *types.Block) ([]types.Tweak, error) {
	// moved outside of function avoid issues with benchmarking
	//common.InfoLogger.Println("Computing tweaks...")
	var tweaks []types.Tweak

	semaphore := make(chan struct{}, common.MaxParallelTweakComputations)

	var errG error
	var muTweaks sync.Mutex // Mutex to protect tweaks
	var muErr sync.Mutex    // Mutex to protect err

	var wg sync.WaitGroup
	// block fetcher routine
	for _, tx := range block.Txs {
		if errG != nil {
			common.ErrorLogger.Println(errG)
			break // If an error occurred, break the loop
		}

		semaphore <- struct{}{} // Acquire a slot
		wg.Add(1)               // make the function wait for this slot
		go func(_tx types.Transaction) {
			//start := time.Now()
			defer func() {
				<-semaphore // Release the slot
				wg.Done()
			}()

			for _, vout := range _tx.Vout {
				// only compute tweak for txs with a taproot output
				if vout.ScriptPubKey.Type == "witness_v1_taproot" {
					tweakPerTx, err := ComputeTweakPerTx(_tx)
					if err != nil {
						common.ErrorLogger.Println(err)
						muErr.Lock()
						if errG == nil {
							errG = err // Store the first error that occurs
						}
						muErr.Unlock()
						break
					}
					// we do this check for not eligible transactions like coinbase transactions
					// they are not supposed to throw an error
					// but also don't have a tweak that can be computed
					if tweakPerTx != nil {
						muTweaks.Lock()
						tweaks = append(tweaks, types.Tweak{
							BlockHash:   block.Hash,
							BlockHeight: block.Height,
							Txid:        _tx.Txid,
							Data:        *tweakPerTx,
						})
						muTweaks.Unlock()
					}
					break
				}
			}
		}(tx)
	}

	wg.Wait()
	//common.InfoLogger.Println("Tweaks computed...")
	return tweaks, nil
}
```

### v1 Code
```go
func ComputeTweaksForBlockV1(block *types.Block) ([]types.Tweak, error) {
	//common.InfoLogger.Println("Computing tweaks...")
	var tweaks []types.Tweak

	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			// only compute tweak for txs with a taproot output
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				tweakPerTx, err := ComputeTweakPerTx(tx)
				if err != nil {
					common.ErrorLogger.Println(err)
					return []types.Tweak{}, err
				}
				// we do this check for not eligible transactions like coinbase transactions
				// they are not supposed to throw an error
				// but also don't have a tweak that can be computed
				if tweakPerTx != nil {
					tweaks = append(tweaks, types.Tweak{
						BlockHash:   block.Hash,
						BlockHeight: block.Height,
						Txid:        tx.Txid,
						Data:        *tweakPerTx,
					})
				}
				break
			}
		}
	}
	//common.InfoLogger.Println("Tweaks computed...")
	return tweaks, nil
}
```


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