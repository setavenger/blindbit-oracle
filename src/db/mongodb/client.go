package mongodb

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sort"
)

// BulkCheckHeadersExist returns nil if all blockHashes are in the database.
// If not all blockHashes are in the db it returns the next notfound height.
func BulkCheckHeadersExist(blockHeaders []types.BlockHeader) (*uint32, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	if len(blockHeaders) == 0 {
		common.WarningLogger.Println("no block_hashes were given")
		return nil, err
	}
	// check whether we still have a light utxo for that txid
	coll := client.Database("headers").Collection("headers")
	// Define your array of txids you want to query

	var blockHashes []string
	for _, header := range blockHeaders {
		blockHashes = append(blockHashes, header.Hash)
	}

	// Create a filter to match documents with txid in the txids array
	filter := bson.M{"hash": bson.M{"$in": blockHashes}}

	common.InfoLogger.Println("Checking for block_hashes...")
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	var results []types.BlockHeader
	if err = cursor.All(context.TODO(), &results); err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	if len(results) == len(blockHashes) {
		// we return nil as no continuation is necessary and all blocks wer processed
		return nil, nil
	}

	var startingPoint uint32

	if len(results) == 0 {
		startingPoint = blockHeaders[0].Height // just to start from somewhere

		// find the lowest possible block that we checked
		// todo can this be omitted if we can guarantee that blockHeaders will always be in order
		for _, header := range blockHeaders {
			if header.Height < startingPoint {
				startingPoint = header.Height
			}
		}
	} else {

		// Sorting the slice by height
		sort.Slice(results, func(i, j int) bool {
			return results[i].Height < results[j].Height
		})

		for i := 0; i < len(results)-1; i++ {
			//common.InfoLogger.Println(i)
			// Check if the next number is more than 1 greater than the current number
			if results[i+1].Height-results[i].Height > 1 {
				// Return the first number in the gap
				startingPoint = results[i].Height + 1
				break
			}
			if i == len(results)-2 {
				// if no header has been found there was no gap, and we just return based on the last element
				// add 2 to skip last element and reach the actual missing height
				startingPoint = results[i].Height + 2
			}

		}
	}

	// double check that highest height was actually set
	if startingPoint == 0 {
		errMsg := "height could not be properly determined. should not happen"
		common.ErrorLogger.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	return &startingPoint, nil

}
