package mongodb

import (
	"SilentPaymentAppBackend/src/common"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// todo add unique lock
//  db.members.createIndex( { groupNumber: 1, lastname: 1, firstname: 1 }, { unique: true } )

func CreateIndices() {
	// todo might be possible to remove the _id_ indexes to save on memory
	//  as there is no plan to query based on the mongodb assigned id
	common.InfoLogger.Println("creating database indices")
	//CreateIndexTransactions()
	CreateIndexCFilters()
	CreateIndexTweaks()
	CreateIndexUTXOs()
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
			"blockheader": 1,
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

// CreateIndexUTXOs will panic because it only runs on startup and should be executed
func CreateIndexUTXOs() {
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
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "block_hash", Value: 1},
			{Key: "block_height", Value: 1},
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
			{Key: "height", Value: 1},
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
	common.InfoLogger.Println("Taproot Filter inserted", "ID", result.InsertedID)
	return nil
}

func BulkInsertSpentUTXOs(utxos []common.SpentUTXO) error {
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

	common.DebugLogger.Printf("Bulk inserted %d new spent utxos\n", len(result.InsertedIDs))
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

	common.InfoLogger.Printf("bulk inserted %d new light utxos\n", len(result.InsertedIDs))
	return nil
}

func BulkInsertTweaks(tweaks []common.Tweak) error {
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

	common.InfoLogger.Printf("bulk inserted %d new tweaks\n", len(result.InsertedIDs))
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

	var writes []mongo.WriteModel
	for _, spentUTXO := range spentUTXOs {
		filter := bson.D{{"txid", spentUTXO.Txid}, {"vout", spentUTXO.Vout}}
		model := mongo.NewDeleteOneModel().SetFilter(filter)
		writes = append(writes, model)
	}

	opts := options.BulkWrite().SetOrdered(false) // SetOrdered(false) allows operations to be executed in parallel, improving performance
	result, err := coll.BulkWrite(context.TODO(), writes, opts)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	common.InfoLogger.Printf("Deleted %d LightUTXOs\n", result.DeletedCount)

	common.InfoLogger.Println("Attempting cut through")
	// todo can this be outsourced into a go routine
	//  does this take too long?
	for _, spentUTXO := range spentUTXOs {
		err = chainedTweakDeletion(client, spentUTXO.Txid)
		if err != nil {
			common.DebugLogger.Printf("Failed on: %+v\n", spentUTXO)
			common.ErrorLogger.Println(err)
			return err
		}
	}

	return nil
}

// chainedTweakDeletion chained deletion of tweak data if no more utxos with a certain txid are left
// runs whenever a light UTXO is deleted in order to keep the database lean and remove unneeded tweaks
func chainedTweakDeletion(client *mongo.Client, txId string) error {
	// check whether we still have a light utxo for that txid
	coll := client.Database("transaction_outputs").Collection("unspent")
	filter := bson.D{{"txid", txId}}
	result := coll.FindOne(context.TODO(), filter)

	var utxo common.LightUTXO

	err := result.Decode(&utxo)
	// "no documents" means none was found and is
	if err != nil && err.Error() != "mongo: no documents in result" {
		common.ErrorLogger.Println(err)
		return err
	}

	// we exit because we found an UTXO if none was found it wouldn't have a txid
	if utxo.TxId != "" {
		return err
	}

	// no match was found, so we delete the tweak data based on the txid
	var resultDelete *mongo.DeleteResult
	coll = client.Database("tweak_data").Collection("tweaks")
	resultDelete, err = coll.DeleteOne(context.TODO(), filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	if resultDelete.DeletedCount == 1 {
		common.DebugLogger.Printf("Deleted tweak data for %s\n", txId)
	} else if resultDelete.DeletedCount > 1 {
		common.DebugLogger.Printf("Deleted %d tweak data for %s\n", resultDelete.DeletedCount, txId)
		common.WarningLogger.Println("this should not happen")
	}

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
