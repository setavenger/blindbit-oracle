package mongodb

import (
	"SilentPaymentAppBackend/src/common"
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sort"
)

// todo add unique lock
//  db.members.createIndex( { groupNumber: 1, lastname: 1, firstname: 1 }, { unique: true } )

func CreateIndices() {
	// todo might be possible to remove the _id_ indexes to save on memory
	//  as there is no plan to query based on the mongodb assigned id
	common.InfoLogger.Println("creating database indices")
	CreateIndexCFilters()
	CreateIndexTweaks()
	CreateIndexLightUTXOs()
	CreateIndexSpentTXOs()
	CreateIndexHeaders()
	common.InfoLogger.Println("created database indices")
}

// CreateIndexCFilters will panic because it only runs on startup and should be executed
func CreateIndexCFilters() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("filters").Collection("taproot")
	indexModel := mongo.IndexModel{
		Keys: bson.M{
			// in rare case counting is off we can then reindex from local DB data
			"block_hash": 1, // todo is it enough to just check the blockHash?
		},
		Options: options.Index().SetUnique(true),
	}
	nameIndex, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}

	common.DebugLogger.Println("Created Index with name:", nameIndex)
}

// CreateIndexLightUTXOs will panic because it only runs on startup and should be executed
func CreateIndexLightUTXOs() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("unspent")
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "txid", Value: 1},
			{Key: "vout", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	nameIndex, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}

	common.DebugLogger.Println("Created Index with name:", nameIndex)

	indexModel = mongo.IndexModel{
		Keys: bson.D{
			{Key: "txid", Value: 1},
		},
	}
	nameIndex, err = coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}
	common.DebugLogger.Println("Created Index with name:", nameIndex)

	indexModel = mongo.IndexModel{
		Keys: bson.D{
			{Key: "tx_id_vout", Value: 1},
		},
	}
	nameIndex, err = coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}
	common.DebugLogger.Println("Created Index with name:", nameIndex)

}

// CreateIndexSpentTXOs will panic because it only runs on startup and should be executed
func CreateIndexSpentTXOs() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("spent")
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "txid", Value: 1},
			{Key: "vout", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	nameIndex, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}
	common.DebugLogger.Println("Created Index with name:", nameIndex)
}

// CreateIndexTweaks will panic because it only runs on startup and should be executed
func CreateIndexTweaks() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("tweak_data").Collection("tweaks")
	//indexModel := mongo.IndexModel{
	//	Keys: bson.D{
	//		{Key: "block_hash", Value: 1},
	//		{Key: "block_height", Value: 1},
	//		{Key: "txid", Value: 1},
	//	},
	//	Options: options.Index().SetUnique(true),
	//}
	//nameIndex, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	//if err != nil {
	//	// will panic because it only runs on startup and should be executed
	//	panic(err)
	//}
	//common.DebugLogger.Println("Created Index with name:", nameIndex)

	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "txid", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	nameIndex, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}
	common.DebugLogger.Println("Created Index with name:", nameIndex)
}

// CreateIndexHeaders will panic because it only runs on startup and should be executed
func CreateIndexHeaders() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("headers").Collection("headers")
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "hash", Value: 1},
			//{Key: "height", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	nameIndex, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		// will panic because it only runs on startup and should be executed
		panic(err)
	}
	common.DebugLogger.Println("Created Index with name:", nameIndex)
}

func SaveFilterTaproot(filter *common.Filter) error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
			return
		}
	}()

	coll := client.Database("filters").Collection("taproot")

	result, err := coll.InsertOne(context.TODO(), filter)
	if err != nil {
		if we, ok := err.(mongo.WriteException); ok {
			for _, writeError := range we.WriteErrors {
				if writeError.Code == 11000 {
					common.DebugLogger.Println(err)
					continue
				} else {
					common.ErrorLogger.Println(err)
					return err
				}
			}
		} else {
			common.ErrorLogger.Println(err)
			return err
		}
	}

	if result == nil {
		return nil
	}
	common.InfoLogger.Println("Taproot Filter inserted")
	return nil
}

func BulkInsertSpentUTXOs(utxos []common.SpentUTXO) error {
	common.InfoLogger.Println("Inserting spent utxos")

	// empty blocks
	if len(utxos) == 0 {
		common.WarningLogger.Println("no spent utxos to insert")
		return nil
	}
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("spent")

	// Convert []*common.SpentUTXO to []interface{}
	var interfaceSlice []interface{}
	for _, utxo := range utxos {
		interfaceSlice = append(interfaceSlice, utxo)
	}

	opts := options.InsertMany().SetOrdered(false)
	result, err := coll.InsertMany(context.TODO(), interfaceSlice, opts)
	if err != nil {
		// Check if the error is a BulkWriteException
		if bwe, ok := err.(mongo.BulkWriteException); ok {
			// Handle each write error individually
			for _, we := range bwe.WriteErrors {
				// Check if the error is due to a duplicate key
				if we.Code == 11000 {
					// Ignore the duplicate key error
					continue
				}
				// Handle other types of write errors
				common.ErrorLogger.Println(we)
				return we
			}
		} else {
			// If the error is not a BulkWriteException, handle it as usual
			common.ErrorLogger.Println(err)
			return err
		}
	}

	common.InfoLogger.Printf("Bulk inserted %d new spent utxos\n", len(result.InsertedIDs))
	return nil
}

func BulkInsertHeaders(headers []common.BlockHeader) error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("headers").Collection("headers")

	// Convert []*common.Header to []interface{}
	var interfaceHeaders []interface{}
	for _, header := range headers {
		interfaceHeaders = append(interfaceHeaders, header)
	}

	result, err := coll.InsertMany(context.TODO(), interfaceHeaders)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	common.DebugLogger.Printf("Bulk inserted %d new headers\n", len(result.InsertedIDs))
	return nil
}

func BulkInsertLightUTXOs(utxos []*common.LightUTXO) error {
	common.InfoLogger.Println("Inserting light utxos")
	// empty blocks happen
	if len(utxos) == 0 {
		common.WarningLogger.Println("no light utxos")
		return nil
	}
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("unspent")

	// Convert []*common.LightUTXO to []interface{}
	var interfaceSlice []interface{}
	for _, utxo := range utxos {
		interfaceSlice = append(interfaceSlice, utxo)
	}

	opts := options.InsertMany().SetOrdered(false)
	result, err := coll.InsertMany(context.TODO(), interfaceSlice, opts)
	if err != nil {
		// Check if the error is a BulkWriteException
		if bwe, ok := err.(mongo.BulkWriteException); ok {
			// Handle each write error individually
			for _, we := range bwe.WriteErrors {
				// Check if the error is due to a duplicate key
				if we.Code == 11000 {
					// Ignore the duplicate key error
					continue
				}
				// Handle other types of write errors
				common.ErrorLogger.Println(we)
				return we
			}
		} else {
			// If the error is not a BulkWriteException, handle it as usual
			common.ErrorLogger.Println(err)
			return err
		}
	}

	common.InfoLogger.Printf("Bulk inserted %d new light utxos\n", len(result.InsertedIDs))
	return nil
}

func BulkInsertTweaks(tweaks []common.Tweak) error {
	common.InfoLogger.Println("Inserting tweaks...")
	// empty blocks happen
	if len(tweaks) == 0 {
		common.WarningLogger.Println("no tweaks to insert")
		return nil
	}
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("tweak_data").Collection("tweaks")

	// Convert []*common.LightUTXO to []interface{}
	var interfaceSlice []interface{}
	for _, tweak := range tweaks {
		interfaceSlice = append(interfaceSlice, tweak)
	}

	opts := options.InsertMany().SetOrdered(false)
	result, err := coll.InsertMany(context.TODO(), interfaceSlice, opts)
	if err != nil {
		// Check if the error is a BulkWriteException
		if bwe, ok := err.(mongo.BulkWriteException); ok {
			// Handle each write error individually
			for _, we := range bwe.WriteErrors {
				// Check if the error is due to a duplicate key
				if we.Code == 11000 {
					// Ignore the duplicate key error
					continue
				}
				// Handle other types of write errors
				common.ErrorLogger.Println(we)
				return we
			}
		} else {
			// If the error is not a BulkWriteException, handle it as usual
			common.ErrorLogger.Println(err)
			return err
		}
	}

	common.InfoLogger.Printf("Bulk inserted %d new tweaks\n", len(result.InsertedIDs))
	return nil
}

func RetrieveLastHeader() (*common.BlockHeader, error) {
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
	coll := client.Database("headers").Collection("headers")
	var result common.BlockHeader
	filter := bson.D{}                                                // no filter, get all documents
	optionsQuery := options.FindOne().SetSort(bson.D{{"height", -1}}) // sort by height in descending order

	err = coll.FindOne(context.TODO(), filter, optionsQuery).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			common.WarningLogger.Println("no header in db")
			// return genesis block if no header is in the DB
			// todo explore whether it is better to always just write the Genesis block into the db on initial startup
			return &common.GenesisBlock, nil
		}
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return &result, nil
}

func RetrieveLightUTXOsByHeight(blockHeight uint32) ([]*common.LightUTXO, error) {
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
	coll := client.Database("transaction_outputs").Collection("unspent")
	filter := bson.D{{"block_height", blockHeight}}

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	var results []*common.LightUTXO
	if err = cursor.All(context.TODO(), &results); err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return results, err
}

func RetrieveSpentUTXOsByHeight(blockHeight uint32) ([]*common.SpentUTXO, error) {
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
	coll := client.Database("transaction_outputs").Collection("spent")
	filter := bson.D{{"block_height", blockHeight}}

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	var results []*common.SpentUTXO
	if err = cursor.All(context.TODO(), &results); err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return results, nil
}

func RetrieveCFilterByHeight(blockHeight uint32) (*common.Filter, error) {
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
	coll := client.Database("filters").Collection("taproot")
	filter := bson.D{{"block_height", blockHeight}}

	result := coll.FindOne(context.TODO(), filter)
	var cFilter common.Filter

	err = result.Decode(&cFilter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return &cFilter, nil
}

func RetrieveTweakDataByHeight(blockHeight uint32) ([]common.Tweak, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.DebugLogger.Println("height:", blockHeight)
		common.ErrorLogger.Println(err)
		return nil, err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()
	coll := client.Database("tweak_data").Collection("tweaks")
	filter := bson.D{{"block_height", blockHeight}}

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		common.DebugLogger.Println("height:", blockHeight)
		common.ErrorLogger.Println(err)
		return nil, err
	}

	var results []common.Tweak
	if err = cursor.All(context.TODO(), &results); err != nil {
		common.DebugLogger.Println("height:", blockHeight)
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return results, err
}

func DeleteLightUTXOsBatch(spentUTXOs []common.SpentUTXO) error {
	common.InfoLogger.Println("Deleting LightUTXOs")
	// empty blocks happen
	if len(spentUTXOs) == 0 {
		common.WarningLogger.Println("no spent utxos")
		return nil
	}
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("unspent")

	var txidVouts []string
	for _, spentUTXO := range spentUTXOs {
		txidVouts = append(txidVouts, fmt.Sprintf("%s:%d", spentUTXO.Txid, spentUTXO.Vout))
	}

	filter := bson.M{"tx_id_vout": bson.M{"$in": txidVouts}}

	result, err := coll.DeleteMany(context.TODO(), filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	common.InfoLogger.Printf("Deleted %d LightUTXOs\n", result.DeletedCount)

	common.InfoLogger.Println("Attempting cut through")
	var txIds []string                     // todo combine with txidVout slice population
	for _, spentUTXO := range spentUTXOs { // todo might need to be changed to deleteMany
		txIds = append(txIds, spentUTXO.Txid)
	}
	// todo can this be outsourced into a go routine
	//  does this take too long?
	err = chainedTweakDeletion(client, txIds)
	if err != nil {
		if len(spentUTXOs) > 0 {
			// just in case we see a block with no taproot outputs, unlikely, but you never know
			common.DebugLogger.Printf("Deletion failed on block: %s\n", spentUTXOs[0].BlockHash)
		}
		common.ErrorLogger.Println(err)
		return err
	}

	return nil
}

// chainedTweakDeletion chained deletion of tweak data if no more utxos with a certain txid are left
// runs whenever a light UTXO is deleted in order to keep the database lean and remove unneeded tweaks
func chainedTweakDeletion(client *mongo.Client, txIds []string) error {
	if len(txIds) == 0 {
		common.WarningLogger.Println("no txids to delete")
		return nil
	}
	// check whether we still have a light utxo for that txid
	coll := client.Database("transaction_outputs").Collection("unspent")
	// Define your array of txids you want to query

	// Create a filter to match documents with txid in the txids array
	filter := bson.M{"txid": bson.M{"$in": txIds}}

	common.InfoLogger.Println("looking for light utxos...")
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	var results []common.LightUTXO
	if err = cursor.All(context.TODO(), &results); err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.InfoLogger.Println("processed light utxos...")

	foundTxids := make(map[string]bool)

	for _, lightUTXO := range results {
		// Mark the txid as found
		foundTxids[lightUTXO.Txid] = true
	}

	// we exit because we found an UTXO if none was found it wouldn't have a txid
	// List for txids that were not found
	var notFoundTxids []string
	// Check which txids were not found
	for _, txid := range txIds {
		if !foundTxids[txid] {
			notFoundTxids = append(notFoundTxids, txid)
		}
	}

	// remove duplicates
	// notFoundTxidsClean can contain many more possible txids to delete than will be deleted in the end
	// reason: because we don't index the chain from genesis there will be spent utxos
	// for which we don't have an entry in the DB both light UTXOs and tweaks
	uniqueTxidsMap := make(map[string]struct{}) // Use a map to hold unique txIds
	var notFoundTxidsClean []string             // This will hold your deduplicated slice

	for _, txId := range notFoundTxids {
		if _, exists := uniqueTxidsMap[txId]; !exists {
			uniqueTxidsMap[txId] = struct{}{}                     // Mark the txId as seen
			notFoundTxidsClean = append(notFoundTxidsClean, txId) // Add to the deduplicated slice
		}
	}
	if len(notFoundTxidsClean) == 0 {
		common.DebugLogger.Println("all txids cleared")
		return nil
	}

	common.InfoLogger.Println("determined obsolete txids...")

	// no match was found, so we delete the tweak data based on the txid
	coll = client.Database("tweak_data").Collection("tweaks")

	filterD := bson.M{"txid": bson.M{"$in": notFoundTxidsClean}}

	result, err := coll.DeleteMany(context.TODO(), filterD)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	common.InfoLogger.Printf("Deleted %d tweaks from db\n", result.DeletedCount)

	return err
}

func CheckHeaderExists(blockHash string) (bool, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return false, err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			common.ErrorLogger.Println(err)
		}
	}()

	coll := client.Database("headers").Collection("headers")
	var result common.BlockHeader

	// Use the hash to filter the documents
	filter := bson.D{{"hash", blockHash}}

	err = coll.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			common.DebugLogger.Println("header not in db yet")
			return false, nil
		}
		common.ErrorLogger.Println(err)
		return false, err
	}

	// A document with the given hash exists
	return true, nil
}

// BulkCheckHeadersExist returns nil if all blockHashes are in the database.
// If not all blockHashes are in the db it returns the next notfound height.
func BulkCheckHeadersExist(blockHeaders []common.BlockHeader) (*uint32, error) {
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

	var results []common.BlockHeader
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
