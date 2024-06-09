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
	common.DebugLogger.Println("header-inv inserted")
	return nil
}

func InsertBatchBlockHeaderInv(headersInv []types.BlockHeaderInv) error {
	common.DebugLogger.Println("Inserting headers-inv...")

	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(headersInv))

	// Convert each HeaderInv to a Pair and assign it to the new slice
	for i, pair := range headersInv {
		pairHelp := pair
		pairs[i] = &pairHelp // todo fix assigning the pointer writes the same value all the time
	}

	err := insertBatch(HeadersInvDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	//for _, headerInv := range headersInv {
	//	common.InfoLogger.Printf("Inserted height: %d\n", headerInv.Height)
	//}
	common.DebugLogger.Printf("Inserted %d headers-inv\n", len(headersInv))
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
	iter := HeadersInvDB.NewIterator(nil, nil)
	defer iter.Release()
	var result types.BlockHeaderInv

	ok := iter.Last()
	if !ok {
		return nil, NoEntryErr{}
	}

	// Process the last element first, then continue with previous elements.
	for {
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

		// Move to the previous entry
		if !iter.Prev() {
			break
		}
	}

	//for iter.Prev() {
	//	// Deserialize data first
	//	err := result.DeSerialiseData(iter.Value())
	//	if err != nil {
	//		common.ErrorLogger.Println(err)
	//		return nil, err
	//	}
	//	if result.Flag == flag {
	//		err = result.DeSerialiseKey(iter.Key())
	//		if err != nil {
	//			common.ErrorLogger.Println(err)
	//			return nil, err
	//		}
	//		break
	//	}
	//}

	err := iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return &result, err
}

// GetMissingHeadersInv looks for all missing BlockerHeadersInv heights in the range of max_height of heights and min_height
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

	iter := HeadersInvDB.NewIterator(&util.Range{Start: minHeightBuf.Bytes(), Limit: maxHeightBuf.Bytes()}, nil)
	defer iter.Release()
	var pairs []types.BlockHeaderInv

	// go backwards, it is more likely that we will be looking for recent blocks
	ok := iter.Last()
	if !ok {
		return heights, nil
	}

	for {
		pair := types.BlockHeaderInv{}
		// we only need the key for the height
		err = pair.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		pairs = append(pairs, pair)

		// Move to the previous entry
		if !iter.Prev() {
			break
		}
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

// GetMissingHeadersInvFlag looks for all missing BlockerHeadersInv heights with a certain flag
// in the range of max_height of heights and min_height according to a
func GetMissingHeadersInvFlag(heights []uint32, flag bool) ([]uint32, error) {
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

	iter := HeadersInvDB.NewIterator(&util.Range{Start: minHeightBuf.Bytes(), Limit: maxHeightBuf.Bytes()}, nil)
	defer iter.Release()
	var pairs []types.BlockHeaderInv

	// go backwards, it is more likely that we will be looking for recent blocks
	ok := iter.Last()
	if !ok {
		return heights, nil
	}

	for {
		pair := types.BlockHeaderInv{}
		// Deserialize data first
		err = pair.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}

		// need the inverse of the flag
		// we throw out all of those that match below
		if pair.Flag == !flag {
			err = pair.DeSerialiseKey(iter.Key())
			if err != nil {
				common.ErrorLogger.Println(err)
				return nil, err
			}
			pairs = append(pairs, pair)
		}
		// Move to the previous entry
		if !iter.Prev() {
			break
		}
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

// FetchAllHeadersInv returns all types.BlockHeaderInv in the DB
func FetchAllHeadersInv() ([]types.BlockHeaderInv, error) {
	pairs, err := retrieveAll(HeadersInvDB, types.PairFactoryBlockHeaderInv)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		common.WarningLogger.Println("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.BlockHeaderInv, len(pairs))
	// Convert each Pair to a kHeaderInv and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.BlockHeaderInv); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	return result, err
}
