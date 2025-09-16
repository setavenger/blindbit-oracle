package dbpebble

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/go-bip352"
)

// compute

// iterate over range of compute indexes
// for each compute index, get the tweak and outputs
// for each output, get the output pubkey
// for each output pubkey, get the label
// if the label is not nil, add the output to the found outputs
// if the label is nil, add the output to the found outputs
// return the found outputs

var (
	scanSecretKey [32]byte
	spendPubKey   *[33]byte

	ErrNoComputeIndex = errors.New("no compute index found")
)

func init() {
	rand.Read(scanSecretKey[:])
	spendPubKey = bip352.PubKeyFromSecKey(&scanSecretKey)
}

func (s *Store) DBComputeComputeIndex(
	ctx context.Context,
	startHeight, endHeight uint32,
) (
	foundOutputs []*FoundOutputShort, err error,
) {
	lb, ub := BoundsComputeIndex(startHeight, endHeight)
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		logging.L.Err(err).
			Uint32("startHeight", startHeight).
			Uint32("endHeight", endHeight).
			Msg("error getting compute index iterator")
		return nil, err
	}
	defer it.Close()
	if !it.First() {
		logging.L.Warn().
			Uint32("startHeight", startHeight).
			Uint32("endHeight", endHeight).
			Msg("no compute index found")
		return nil, nil
	}

	counter := 0

	logging.L.Debug().Msgf("Processing compute indexes from %d to %d", startHeight, endHeight)
	for ok := it.First(); ok; ok = it.Next() {
		select {
		case <-ctx.Done():
			logging.L.Info().Int("counter", counter).Msg("Context done")
			return nil, ctx.Err()
		default:
		}
		counter++
		data := it.Value()
		var tweak [33]byte
		copy(tweak[:], data[:SizeTweak])

		countOutputs := len(data[SizeTweak:]) / 8
		shortOutputs := make([][8]byte, countOutputs)
		outputs := data[SizeTweak:]
		for i := range countOutputs {
			copy(shortOutputs[i][:], outputs[i*8:(i+1)*8])
		}
		var foundPerTx []*FoundOutputShort
		foundPerTx, err = ReceiverScanTransactionShortOutputs(
			scanSecretKey, spendPubKey, nil, shortOutputs, &tweak, nil,
		)
		if err != nil {
			return nil, err
		}
		if len(foundPerTx) > 0 {
			keyData := it.Key()
			keyData = keyData[1:] // remove the prefix
			height := binary.BigEndian.Uint32(keyData[:SizeHeight])
			txid := keyData[SizeHeight:]
			logging.L.Debug().Msgf(
				"Found %d outputs at height %d and txid %x",
				len(foundPerTx), height, utils.ReverseBytesCopy(txid[:]),
			)

			// Set the txid for each found output
			for _, found := range foundPerTx {
				copy(found.Txid[:], txid)
			}
			foundOutputs = append(foundOutputs, foundPerTx...)
			// <-time.After(10 * time.Second)
		}
	}

	return foundOutputs, nil
}

// WorkRange represents a range of block heights to process
type WorkRange struct {
	startHeight uint32
	endHeight   uint32
	lowerBound  []byte
	upperBound  []byte
}

// WorkStealingQueue manages work ranges for parallel processing
type WorkStealingQueue struct {
	ranges []WorkRange
	index  int64
}

// ProgressTracker tracks overall progress across all workers
type ProgressTracker struct {
	totalProcessed int64
	totalRanges    int
	mu             sync.RWMutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(totalRanges int) *ProgressTracker {
	return &ProgressTracker{
		totalRanges: totalRanges,
	}
}

// UpdateProgress atomically updates the progress and logs if needed
func (pt *ProgressTracker) UpdateProgress(processed int64) {
	pt.mu.Lock()
	pt.totalProcessed += processed
	current := pt.totalProcessed
	total := int64(pt.totalRanges)
	pt.mu.Unlock()

	// Log every 12.5% progress
	logInterval := max(total/8, 1)
	if current%logInterval == 0 {
		percentage := float64(current) / float64(total) * 100
		logging.L.Info().
			Int64("processedRanges", current).
			Int64("totalRanges", total).
			Float64("percentage", percentage).
			Msg("Progress")
	}
}

// GetProgress returns current progress
func (pt *ProgressTracker) GetProgress() (processed int64, total int64) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.totalProcessed, int64(pt.totalRanges)
}

// NewWorkStealingQueue creates a new work stealing queue with ranges of specified size
func NewWorkStealingQueue(startHeight, endHeight uint32, rangeSize uint32) *WorkStealingQueue {
	var ranges []WorkRange

	for current := startHeight; current < endHeight; current += rangeSize {
		rangeEnd := min(current+rangeSize, endHeight)

		lb, ub := BoundsComputeIndex(current, rangeEnd)
		ranges = append(ranges, WorkRange{
			startHeight: current,
			endHeight:   rangeEnd,
			lowerBound:  lb,
			upperBound:  ub,
		})
	}

	return &WorkStealingQueue{
		ranges: ranges,
		index:  0,
	}
}

// GetNextRange atomically gets the next available work range
func (q *WorkStealingQueue) GetNextRange() *WorkRange {
	currentIndex := atomic.AddInt64(&q.index, 1) - 1
	if int(currentIndex) >= len(q.ranges) {
		return nil // No more work
	}
	return &q.ranges[currentIndex]
}

// DBComputeComputeIndexParallel processes compute indexes using work-stealing parallelization
func (s *Store) DBComputeComputeIndexParallel(
	ctx context.Context,
	startHeight, endHeight uint32,
	numWorkers int,
	rangeSize uint32, // Size of each range in block heights (e.g., 144)
) ([]*FoundOutputShort, error) {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	if rangeSize <= 0 {
		rangeSize = 144 // Default to 144 blocks per range
	}

	// Create work stealing queue
	workQueue := NewWorkStealingQueue(startHeight, endHeight, rangeSize)

	// Create progress tracker
	progressTracker := NewProgressTracker(len(workQueue.ranges))

	logging.L.Info().
		Uint32("startHeight", startHeight).
		Uint32("endHeight", endHeight).
		Int("numWorkers", numWorkers).
		Msg("Starting parallel processing")

	// Create channels for results and errors
	results := make(chan []*FoundOutputShort, numWorkers)
	errorChan := make(chan error, numWorkers)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < min(numWorkers, len(workQueue.ranges)); i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			workerCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			var workerResults []*FoundOutputShort
			processedRanges := 0

			for {
				select {
				case <-workerCtx.Done():
					return
				default:
				}

				// Get next work range
				workRange := workQueue.GetNextRange()
				if workRange == nil {
					break // No more work
				}

				processedRanges++

				// Call the existing function with the range bounds
				rangeResults, err := s.DBComputeComputeIndex(
					workerCtx, workRange.startHeight, workRange.endHeight,
				)
				if err != nil {
					logging.L.Err(err).
						Int("workerID", workerID).
						Uint32("startHeight", workRange.startHeight).
						Uint32("endHeight", workRange.endHeight).
						Msg("Error processing range")
					errorChan <- err
					return
				}

				workerResults = append(workerResults, rangeResults...)

				// Update progress (each range = 1 unit of progress)
				progressTracker.UpdateProgress(1)
			}

			results <- workerResults
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()

	// Close channels after all workers are done
	close(results)

	// Collect all results
	var allFoundOutputs []*FoundOutputShort
	resultsCollected := 0

	for workerResults := range results {
		allFoundOutputs = append(allFoundOutputs, workerResults...)
		resultsCollected++
	}

	// Check for errors - exit on first error
	select {
	case err := <-errorChan:
		logging.L.Err(err).Msg("Error during processing")
		return nil, err
	default:
	}

	logging.L.Info().
		Int("totalFoundOutputs", len(allFoundOutputs)).
		Msg("Processing completed")

	close(errorChan)

	return allFoundOutputs, nil
}

// ------ below belongs to other packages ------

type FoundOutputShort struct {
	// Only first 8 bytes
	Output      [8]byte
	SecKeyTweak [32]byte
	Label       *bip352.Label
	Txid        [32]byte
}

func (f *FoundOutputShort) GetOutput() [8]byte {
	return f.Output
}

func (f *FoundOutputShort) GetSecKeyTweak() [32]byte {
	return f.SecKeyTweak
}

func (f *FoundOutputShort) GetLabel() *bip352.Label {
	return f.Label
}

// ReceiverScanTransactionShortOutputs scans but with 8 byte outputs instead fo full outputs
// scanKey: scanning secretKey of the receiver
//
// receiverSpendPubKey: spend pubKey of the receiver
//
// txOutputs: x-only outputs of the specific transaction
//
// labels: existing label public keys as bytes [wallets should always check for the change label]
//
// publicComponent: either A_sum or tweaked (A_sum * input_hash);
// if already tweaked the inputHash should be nil or the computation will be flawed
//
// inputHash: 32 byte can be nil if publicComponent is a tweak and already includes the input_hash
func ReceiverScanTransactionShortOutputs(
	scanKey [32]byte,
	receiverSpendPubKey *[33]byte,
	labels []*bip352.Label,
	txOutputs [][8]byte, // 8 byte short outputs only first bytes
	publicComponent *[33]byte,
	inputHash *[32]byte,
) ([]*FoundOutputShort, error) {
	// todo should probably check inputs before computation especially the labels
	var foundOutputs []*FoundOutputShort

	sharedSecret, err := bip352.CreateSharedSecret(publicComponent, &scanKey, inputHash)
	if err != nil {
		return nil, err
	}

	var k uint32 = 0
	for {
		var outputPubKey [32]byte
		var tweak [32]byte
		outputPubKey, tweak, err = bip352.CreateOutputPubKeyTweak(sharedSecret, receiverSpendPubKey, k)
		if err != nil {
			return nil, err
		}

		var found bool
		for i := range txOutputs {
			// only check the first 8 bytes of the txOutput and outputPubKey
			if bytes.Equal(outputPubKey[:8], txOutputs[i][:]) {
				foundOutputs = append(foundOutputs, &FoundOutputShort{
					Output:      txOutputs[i],
					SecKeyTweak: tweak,
					Label:       nil,
				})
				// txOutputs = slices.Delete(txOutputs, i, i+1) // very slow
				txOutputs = append(txOutputs[:i], txOutputs[i+1:]...)
				found = true
				k++
				break // found the matching txOutput for outputPubKey, don't try the rest
			}

			if labels == nil {
				continue
			}

			// now check the labels
			var foundLabel *bip352.Label

			// todo: benchmark against
			// var prependedTxOutput [33]byte
			// prependedTxOutput[0] = 0x02
			// copy(prependedTxOutput[1:], txOutput[:])

			prependedTxOutput := utils.ConvertToFixedLength33(append([]byte{0x02}, txOutputs[i][:]...))
			prependedOutputPubKey := utils.ConvertToFixedLength33(append([]byte{0x02}, outputPubKey[:]...))

			// start with normal output
			foundLabel, err = MatchLabels(prependedTxOutput, prependedOutputPubKey, labels)
			if err != nil {
				return nil, err
			}

			// important: copy the tweak to avoid modifying the original tweak
			var secKeyTweak [32]byte
			copy(secKeyTweak[:], tweak[:])

			if foundLabel != nil {
				err = bip352.AddPrivateKeys(&secKeyTweak, &foundLabel.Tweak) // labels have a modified tweak
				if err != nil {
					return nil, err
				}
				foundOutputs = append(foundOutputs, &FoundOutputShort{
					Output:      txOutputs[i],
					SecKeyTweak: secKeyTweak,
					Label:       foundLabel,
				})
				txOutputs = append(txOutputs[:i], txOutputs[i+1:]...)
				found = true
				k++
				break
			}

			// try the negated output for the label
			err = bip352.NegatePublicKey(&prependedTxOutput)
			if err != nil {
				return nil, err
			}

			foundLabel, err = MatchLabels(prependedTxOutput, prependedOutputPubKey, labels)
			if err != nil {
				return nil, err
			}
			if foundLabel != nil {
				err = bip352.AddPrivateKeys(&secKeyTweak, &foundLabel.Tweak) // labels have a modified tweak
				if err != nil {
					return nil, err
				}
				foundOutputs = append(foundOutputs, &FoundOutputShort{
					Output:      [8]byte(prependedTxOutput[1:9]), // 8 bytes
					SecKeyTweak: secKeyTweak,
					Label:       foundLabel,
				})
				txOutputs = append(txOutputs[:i], txOutputs[i+1:]...)
				found = true
				k++
				break
			}
		}

		if !found {
			break
		}
	}
	return foundOutputs, nil
}

func MatchLabels(txOutput, pk [33]byte, labels []*bip352.Label) (*bip352.Label, error) {
	var pkNeg [33]byte
	copy(pkNeg[:], pk[:])
	// subtraction is adding a negated value
	err := bip352.NegatePublicKey(&pkNeg)
	if err != nil {
		return nil, err
	}

	// todo: is this the best place to prepend to compressed
	labelMatch, err := bip352.AddPublicKeys(&txOutput, &pkNeg)
	if err != nil {
		return nil, err
	}

	for _, label := range labels {
		// only check the first 8 bytes of actual pubkey
		if bytes.Equal(labelMatch[1:8+1], label.PubKey[1:8+1]) {
			return label, nil
		}
	}

	return nil, nil
}
