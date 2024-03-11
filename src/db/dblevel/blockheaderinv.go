package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"bytes"
	"encoding/binary"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func InsertBlockHeaderInv(pair types.BlockHeaderInv) error {
	err := insertSimple(HeadersInvDB, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.InfoLogger.Println("header-inv inserted")
	return nil
}

func InsertBatchBlockHeaderInv(headersInv []types.BlockHeaderInv) error {
	common.InfoLogger.Println("Inserting headers-inv...")

	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(headersInv))

	// Convert each HeaderInv to a Pair and assign it to the new slice
	for i, pair := range headersInv {
		pairs[i] = &pair
	}

	err := insertBatch(HeadersInvDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.InfoLogger.Printf("Inserted %d tweaks", len(headersInv))
	return nil
}

func FetchByBlockHeightBlockHeaderInv(height uint32) (types.BlockHeaderInv, error) {
	var pair types.BlockHeaderInv
	err := retrieveByBlockHeight(HeadersInvDB, height, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return types.BlockHeaderInv{}, err
	}
	return pair, nil
}

// FetchHighestBlockHeaderInv fetches the header with the highest key regardless of the flag
func FetchHighestBlockHeaderInv() (*types.BlockHeaderInv, error) {
	iter := HeadersInvDB.NewIterator(nil, nil)
	defer iter.Release()
	var result types.BlockHeaderInv

	if iter.Last() {
		// Deserialize data first
		err := result.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		err = result.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
	}

	err := iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if result.Hash == "" {
		common.WarningLogger.Println("no entry found")
		return nil, NoEntryErr{}
	}
	return &result, err
}

// FetchHighestBlockHeaderInvByFlag gets the block with the highest height which has the corresponding flag set
// Flag being either processed or unprocessed according to types.BlockHeaderInv
func FetchHighestBlockHeaderInvByFlag(flag bool) (*types.BlockHeaderInv, error) {
	// Create an iterator that iterates in reverse order
	iter := HeadersInvDB.NewIterator(nil, nil)
	defer iter.Release()
	var result types.BlockHeaderInv

	ok := iter.Last()
	if !ok {
		return nil, NoEntryErr{}
	}

	for iter.Prev() {
		// Deserialize data first
		err := result.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		if result.Flag == flag {
			err = result.DeSerialiseKey(iter.Key())
			if err != nil {
				common.ErrorLogger.Println(err)
				return nil, err
			}
			break
		}
	}

	err := iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return &result, err
}

// GetMissingHeadersInv looks for all missing BlockerHeadersInv in the range of max_height of heights and min_height
func GetMissingHeadersInv(heights []uint32) ([]uint32, error) {
	// general combination of heights as input could be simplified
	// to directly compare the keys in the iter against the heights provided
	// keeping it general might not be bad for future use cases
	// keep an eye on performance around this function

	if len(heights) == 0 {
		common.ErrorLogger.Println("passed an empty slice to check")
		return []uint32{}, nil
	}

	var minHeight = heights[0]
	var maxHeight = heights[0]

	for _, height := range heights {
		if height > maxHeight {
			maxHeight = height
			continue // can't be both higher than max and lower than min, skip to save time
		}
		if height < minHeight {
			minHeight = height
		}
	}

	// convert min and max to bytes for range inputs
	var minHeightBuf bytes.Buffer
	err := binary.Write(&minHeightBuf, binary.BigEndian, minHeight)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	var maxHeightBuf bytes.Buffer
	err = binary.Write(&maxHeightBuf, binary.BigEndian, maxHeight)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// Create an iterator that iterates in reverse order
	iter := HeadersInvDB.NewIterator(&util.Range{Start: minHeightBuf.Bytes(), Limit: maxHeightBuf.Bytes()}, nil)
	defer iter.Release()
	var pairs []types.BlockHeaderInv

	for iter.Next() {
		pair := types.BlockHeaderInv{}
		// we only need the key for the height
		err = pair.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		pairs = append(pairs, pair)
	}

	err = iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// shortcut to save some iterations below
	if len(pairs) == 0 {
		return heights, nil
	}

	var unmatchedHeights []uint32
	heightSet := make(map[uint32]bool)

	// Populate the set with heights from blockHeaders
	for _, blockHeader := range pairs {
		heightSet[blockHeader.Height] = true
	}

	// Iterate over the heights array and check each height against the set
	for _, height := range heights {
		if _, found := heightSet[height]; !found {
			unmatchedHeights = append(unmatchedHeights, height)
		}
	}

	return unmatchedHeights, err
}
