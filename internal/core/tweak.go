package core

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/types"
	"github.com/setavenger/go-bip352"
)

func ComputeTweaksForBlock(block *types.Block) ([]types.Tweak, error) {
	// performance tests have shown that for blocks with low transaction count v3 constantly outperforms the other implementations
	if len(block.Txs) < 1000 {
		return ComputeTweaksForBlockV3(block)
	} else {
		//We use v2 until v4 becomes stable and a bit better
		return ComputeTweaksForBlockV2(block)
	}
}

// ComputeTweaksForBlockV4 Upgraded version of v2
func ComputeTweaksForBlockV4(block *types.Block) ([]types.Tweak, error) {
	var tweaks []types.Tweak
	var mu sync.Mutex // Mutex to protect shared resources
	var wg sync.WaitGroup

	// Create channels for transactions and results
	txChannel := make(chan types.Transaction)
	resultsChannel := make(chan types.Tweak)

	semaphore := make(chan struct{}, config.MaxParallelTweakComputations)

	// Start worker goroutines
	for i := 0; i < config.MaxParallelTweakComputations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tx := range txChannel {
				semaphore <- struct{}{} // Acquire a slot
				for _, vout := range tx.Vout {
					if vout.ScriptPubKey.Type == "witness_v1_taproot" {
						tweakPerTx, err := ComputeTweakPerTx(tx)
						if err != nil {
							logging.L.Err(err).Msg("error computing tweak per tx")
							// todo errG
							break
						}
						if tweakPerTx != nil {
							tweakPerTx.BlockHash = block.Hash
							tweakPerTx.BlockHeight = block.Height
							resultsChannel <- *tweakPerTx
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

// ComputeTweaksForBlockV3 performs worse for high tx count but faster for low tx count <800-1000 txs
func ComputeTweaksForBlockV3(block *types.Block) ([]types.Tweak, error) {
	if block.Txs == nil || len(block.Txs) == 0 {
		logging.L.Debug().Any("block", block).Msg("Block had zero transactions")
		logging.L.Warn().Msg("Block had zero transactions")
		return []types.Tweak{}, nil
	}
	var tweaks []types.Tweak
	var muTweaks sync.Mutex // Mutex to protect tweaks slice
	var muErr sync.Mutex    // Mutex to protect error
	var wg sync.WaitGroup

	totalTxs := len(block.Txs)
	numGoroutines := config.MaxParallelTweakComputations // Number of goroutines you want to spin up
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
							logging.L.Err(err).Msg("error computing tweak per tx")
							muErr.Lock()
							if errG == nil {
								errG = err // Store the first error that occurs
							}
							muErr.Unlock()
							break
						}
						if tweakPerTx != nil {
							tweakPerTx.BlockHash = block.Hash
							tweakPerTx.BlockHeight = block.Height

							localTweaks = append(localTweaks, *tweakPerTx)
						}
						break
					}
				}
			}

			// Safely append to the global slice
			muTweaks.Lock()
			tweaks = append(tweaks, localTweaks...)
			muTweaks.Unlock()
		}(batch)
	}

	if errG != nil {
		panic(errG)
	}

	wg.Wait()
	return tweaks, nil
}

func ComputeTweaksForBlockV2(block *types.Block) ([]types.Tweak, error) {
	// moved outside of function avoid issues with benchmarking
	//common.InfoLogger.Println("Computing tweaks...")
	var tweaks []types.Tweak

	semaphore := make(chan struct{}, config.MaxParallelTweakComputations)

	var errG error
	var muTweaks sync.Mutex // Mutex to protect tweaks
	var muErr sync.Mutex    // Mutex to protect err

	var wg sync.WaitGroup
	// block fetcher routine
	for _, tx := range block.Txs {
		if errG != nil {
			logging.L.Err(errG).Msg("error computing tweaks")
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
						logging.L.Err(err).Msg("error computing tweak per tx")
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
						tweakPerTx.BlockHash = block.Hash
						tweakPerTx.BlockHeight = block.Height

						muTweaks.Lock()
						tweaks = append(tweaks, *tweakPerTx)
						muTweaks.Unlock()
					}
					break
				}
			}
		}(tx)
	}

	if errG != nil {
		panic(errG)
	}
	wg.Wait()
	//common.InfoLogger.Println("Tweaks computed...")
	return tweaks, nil
}

// Deprecated: slowest of them all, do not use anywhere
func ComputeTweaksForBlockV1(block *types.Block) ([]types.Tweak, error) {
	//common.InfoLogger.Println("Computing tweaks...")
	var tweaks []types.Tweak

	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			// only compute tweak for txs with a taproot output
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				tweakPerTx, err := ComputeTweakPerTx(tx)
				if err != nil {
					logging.L.Err(err).Msg("error computing tweak per tx")
					return []types.Tweak{}, err
				}
				// we do this check for not eligible transactions like coinbase transactions
				// they are not supposed to throw an error
				// but also don't have a tweak that can be computed
				if tweakPerTx != nil {
					tweakPerTx.BlockHash = block.Hash
					tweakPerTx.BlockHeight = block.Height
					tweaks = append(tweaks, *tweakPerTx)
				}
				break
			}
		}
	}
	//common.InfoLogger.Println("Tweaks computed...")
	return tweaks, nil
}

func ComputeTweakPerTx(tx types.Transaction) (*types.Tweak, error) {
	//common.DebugLogger.Println("computing tweak for:", tx.Txid)
	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		// for example if coinbase transaction does not return any pubKeys (as it should)
		return nil, nil
	}
	summedKey, err := sumPublicKeys(pubKeys)
	if err != nil {
		if strings.Contains(err.Error(), "not on secp256k1 curve") {
			logging.L.Warn().Str("txid", tx.Txid).Err(err).Msg("error computing tweak per tx")
			return nil, nil
		}
		logging.L.Debug().Str("txid", tx.Txid).Msg("error computing tweak per tx")
		logging.L.Err(err).Msg("error computing tweak per tx")
		return nil, err
	}
	hash, err := ComputeInputHash(tx, summedKey)
	if err != nil {
		logging.L.Debug().Str("txid", tx.Txid).Msg("error computing tweak per tx")
		logging.L.Err(err).Msg("error computing tweak per tx")
		return nil, err
	}
	curve := btcec.S256()

	x, y := curve.ScalarMult(summedKey.X(), summedKey.Y(), hash[:])

	tweakBytes := [33]byte{}
	mod := y.Mod(y, big.NewInt(2))
	if mod.Cmp(big.NewInt(0)) == 0 {
		tweakBytes[0] = 0x02
	} else {
		tweakBytes[0] = 0x03
	}

	x.FillBytes(tweakBytes[1:])

	highestValue, err := FindBiggestOutputFromTx(tx)
	if err != nil {
		logging.L.Err(err).Msg("error computing tweak per tx")
		return nil, err
	}

	tweak := types.Tweak{
		Txid:         tx.Txid,
		TweakData:    tweakBytes,
		HighestValue: highestValue,
	}

	return &tweak, nil
}

func FindBiggestOutputFromTx(tx types.Transaction) (uint64, error) {
	var biggest uint64

	for _, output := range tx.Vout {
		if output.ScriptPubKey.Type != "witness_v1_taproot" {
			continue
		}
		valueOutput := utils.ConvertFloatBTCtoSats(output.Value)
		if valueOutput > biggest {
			biggest = valueOutput
		}
	}

	if biggest == 0 {
		logging.L.Debug().Any("tx", tx).Msg("highest value was 0")
		logging.L.Err(errors.New("highest value was 0")).Msg("highest value was 0")
	}

	return biggest, nil
}

func extractPubKeys(tx types.Transaction) []string {
	var pubKeys []string

	for _, vin := range tx.Vin {
		if vin.Coinbase != "" {
			continue
		}
		switch vin.Prevout.ScriptPubKey.Type {
		case "witness_v1_taproot":
			// todo needs some extra parsing see reference implementation and bitcoin core wallet
			pubKey, err := extractPubKeyFromP2TR(vin)
			if err != nil {
				logging.L.Debug().Str("txid", tx.Txid).Msg("Could not extract public key")
				logging.L.Panic().Err(err).Msg("Could not extract public key")
				return nil
			}
			// todo what to do if none is matched
			if pubKey != "" {
				pubKeys = append(pubKeys, pubKey)
			}
		case "witness_v0_keyhash":
			// last element in the witness data is public key; skip uncompressed
			if len(vin.Txinwitness[len(vin.Txinwitness)-1]) == 66 {
				pubKeys = append(pubKeys, vin.Txinwitness[len(vin.Txinwitness)-1])
			}

		case "scripthash":
			if len(vin.ScriptSig.Hex) == 46 {
				if vin.ScriptSig.Hex[:6] == "160014" {
					if len(vin.Txinwitness[len(vin.Txinwitness)-1]) == 66 {
						pubKeys = append(pubKeys, vin.Txinwitness[len(vin.Txinwitness)-1])
					}
				}
			}
		case "pubkeyhash":
			pubKey, err := extractFromP2PKH(vin)
			if err != nil {
				logging.L.Debug().Str("txid", tx.Txid).Msg("Could not extract public key")
				logging.L.Err(err).Msg("Could not extract public key")
				continue
			}

			// todo what to do if none is matched
			if pubKey != nil {
				pubKeys = append(pubKeys, hex.EncodeToString(pubKey))
			}

		default:
			continue
		}
	}

	return pubKeys
}

// extractPublicKey tries to find a public key within the given scriptSig.
func extractFromP2PKH(vin types.Vin) ([]byte, error) {
	spkHashHex := vin.Prevout.ScriptPubKey.Hex[6:46] // Skip op_codes and grab the hash
	spkHash, err := hex.DecodeString(spkHashHex)
	if err != nil {
		logging.L.Err(err).Msg("error decoding spk hash")
		return nil, err
	}

	scriptSigBytes, err := hex.DecodeString(vin.ScriptSig.Hex)
	if err != nil {
		logging.L.Err(err).Msg("error decoding script sig")
		return nil, err
	}

	// todo: inefficient implementation copied from reference implementation
	//  should be improved upon
	for i := len(scriptSigBytes); i >= 33; i-- {
		pubKeyBytes := scriptSigBytes[i-33 : i]
		pubKeyHash := bip352.Hash160(pubKeyBytes)
		if bytes.Equal(pubKeyHash, spkHash) {
			return pubKeyBytes, nil
		}
	}

	return nil, nil
}

func extractPubKeyFromP2TR(vin types.Vin) (string, error) {
	witnessStack := vin.Txinwitness

	if len(witnessStack) >= 1 {
		// Remove annex if present
		if len(witnessStack) > 1 && witnessStack[len(witnessStack)-1] == "50" {
			witnessStack = witnessStack[:len(witnessStack)-1]
		}

		if len(witnessStack) > 1 {
			// Script-path spend
			controlBlock, err := hex.DecodeString(witnessStack[len(witnessStack)-1])
			if err != nil {
				logging.L.Err(err).Msg("error decoding control block")
				return "", err
			}
			// Control block format: <control byte> <32-byte internal key> [<32-byte hash>...]
			if len(controlBlock) >= 33 {
				internalKey := controlBlock[1:33]

				if bytes.Equal(internalKey, bip352.NumsH) {
					// Skip if internal key is NUMS_H
					return "", nil
				}

				return vin.Prevout.ScriptPubKey.Hex[4:], nil
			}
		}

		return vin.Prevout.ScriptPubKey.Hex[4:], nil
	}

	return "", nil
}

func sumPublicKeys(pubKeys []string) (*btcec.PublicKey, error) {
	var lastPubKey *btcec.PublicKey
	curve := btcec.KoblitzCurve{}

	for idx, pubKey := range pubKeys {
		bytesPubKey, err := hex.DecodeString(pubKey)
		if err != nil {
			logging.L.Err(err).Msg("error decoding public key")
			// todo remove panics
			return nil, err
		}

		// for extracted keys which are only 32 bytes (taproot) we assume even parity
		// as we don't need the y-coordinate for any computation we can simply prepend 0x02
		if len(bytesPubKey) == 32 {
			bytesPubKey = bytes.Join([][]byte{{0x02}, bytesPubKey}, []byte{})
		}
		publicKey, err := btcec.ParsePubKey(bytesPubKey)
		if err != nil {
			logging.L.Err(err).Msg("error parsing public key")
			return nil, err
		}

		if idx == 0 {
			lastPubKey = publicKey
		} else {
			x, y := curve.Add(lastPubKey.X(), lastPubKey.Y(), publicKey.X(), publicKey.Y())

			lastPubKey, err = bip352.ConvertPointsToPublicKey(x, y)
			if err != nil {
				logging.L.Err(err).Msg("error converting points to public key")
				return nil, err
			}
		}
	}
	return lastPubKey, nil
}

// ComputeInputHash computes the input_hash for a transaction as per the specification.
func ComputeInputHash(tx types.Transaction, sumPublicKeys *btcec.PublicKey) ([32]byte, error) {
	smallestOutpoint, err := findSmallestOutpoint(tx)
	if err != nil {
		logging.L.Err(err).Msg("error finding smallest outpoint")
		return [32]byte{}, err
	}

	// Concatenate outpointL and A
	var buffer bytes.Buffer
	buffer.Write(smallestOutpoint)
	// Serialize the x-coordinate of the sumPublicKeys
	buffer.Write(sumPublicKeys.SerializeCompressed())

	inputHash := bip352.HashTagged("BIP0352/Inputs", buffer.Bytes())

	return inputHash, nil
}

func findSmallestOutpoint(tx types.Transaction) ([]byte, error) {
	if len(tx.Vin) == 0 {
		return nil, errors.New("transaction has no inputs")
	}

	// Define a slice to hold the serialized outpoints
	outpoints := make([][]byte, 0, len(tx.Vin))

	for _, vin := range tx.Vin {
		// Skip coinbase transactions as they do not have a regular prevout
		if vin.Coinbase != "" {
			continue
		}

		// Decode the Txid (hex to bytes) and reverse it to match little-endian format
		txidBytes, err := hex.DecodeString(vin.Txid)
		if err != nil {
			logging.L.Err(err).Msg("error decoding txid")
			return nil, err
		}
		reversedTxid := utils.ReverseBytes(txidBytes)

		// Serialize the Vout as little-endian bytes
		voutBytes := new(bytes.Buffer)
		err = binary.Write(voutBytes, binary.LittleEndian, vin.Vout)
		if err != nil {
			logging.L.Err(err).Msg("error serializing vout")
			return nil, err
		}
		// Concatenate reversed Txid and Vout bytes
		outpoint := append(reversedTxid, voutBytes.Bytes()...)

		// Add the serialized outpoint to the slice
		outpoints = append(outpoints, outpoint)
	}

	// Sort the slice of outpoints to find the lexicographically smallest one
	sort.Slice(outpoints, func(i, j int) bool {
		return bytes.Compare(outpoints[i], outpoints[j]) < 0
	})

	// Return the smallest outpoint, if available
	if len(outpoints) > 0 {
		return outpoints[0], nil
	}

	return nil, errors.New("no valid outpoints found in transaction inputs, should not happen")
}
