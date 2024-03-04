package mongodb

import (
	"SilentPaymentAppBackend/src/common"
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

// todo add unique lock
//  db.members.createIndex( { groupNumber: 1, lastname: 1, firstname: 1 }, { unique: true } )

func CreateIndices() {
	common.InfoLogger.Println("creating database indices")
	CreateIndexTransactions()
	CreateIndexCFilters()
	CreateIndexTweaks()
	CreateIndexUTXOs()
	CreateIndexSpentTXOs()
	CreateIndexHeaders()
	common.InfoLogger.Println("created database indices")
}

// CreateIndexTransactions will panic because it only runs on startup and should be executed
func CreateIndexTransactions() {
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

	coll := client.Database("transactions").Collection("taproot_transactions")
	indexModel := mongo.IndexModel{
		Keys: bson.M{
			"txid": 1,
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

	coll := client.Database("filters").Collection("general")
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
			panic(err)
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
			panic(err)
		}
	}()

	coll := client.Database("tweak_data").Collection("tweaks")
	indexModel := mongo.IndexModel{
		Keys: bson.M{
			"txid": 1,
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
			panic(err)
		}
	}()

	coll := client.Database("headers").Collection("headers")
	indexModel := mongo.IndexModel{
		Keys: bson.M{
			"hash": 1,
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
		//todo don't log duplicate keys as error but rather as debug
		common.ErrorLogger.Println(err)
		return err
	}

	log.Println("Taproot Filter inserted", "ID", result.InsertedID)
	return nil
}

func SaveTweakIndex(tweak common.TweakIndex) error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("tweak_data").Collection("tweaks")

	result, err := coll.InsertOne(context.TODO(), tweak)
	if err != nil {
		common.ErrorLogger.Println(err)
		//panic(err)
		return err
	}

	log.Printf("Tweak inserted with ID: %s\n", result.InsertedID)
	return nil
}

func SaveSpentUTXO(utxo common.SpentUTXO) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("spent")

	result, err := coll.InsertOne(context.TODO(), utxo)
	if err != nil {
		common.ErrorLogger.Println(err)
		return
	}

	log.Printf("Spent Transaction output inserted with ID: %s\n", result.InsertedID)
}

func BulkInsertSpentUTXOs(utxos []common.SpentUTXO) error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("headers").Collection("headers")

	// Convert []*common.Header to []interface{}
	var interfaceHeaders []interface{}
	for _, utxo := range utxos {
		interfaceHeaders = append(interfaceHeaders, utxo)
	}

	result, err := coll.InsertMany(context.TODO(), interfaceHeaders)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	common.DebugLogger.Printf("Bulk inserted %d new headers\n", len(result.InsertedIDs))
	return nil
}

func SaveBulkHeaders(headers []*common.BlockHeader) error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
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

func BulkInsertLightUTXOs(lightUtxos []*common.LightUTXO) error {
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
	var interfaceDocs []interface{}
	for _, header := range lightUtxos {
		interfaceDocs = append(interfaceDocs, header)
	}

	result, err := coll.InsertMany(context.TODO(), interfaceDocs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	log.Printf("bulk inserted %d new light utxos\n", len(result.InsertedIDs))
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
			common.ErrorLogger.Println("no header in db")
			// return genesis block if no header is in the DB
			// todo explore whether it is better to always just write the Genesis block into the db on initial startup
			return &common.GenesisBlock, nil
		}
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return &result, nil
}

func RetrieveHeader(blockHash string) (*common.BlockHeader, error) {
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
	filter := bson.D{{"hash", blockHash}}

	err = coll.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Println("No documents found!")
			return nil, errors.New("no documents found")
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
	filter := bson.D{{"blockheight", blockHeight}}

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
	filter := bson.D{{"blockheight", blockHeight}}

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
	filter := bson.D{{"blockheight", blockHeight}}

	result := coll.FindOne(context.TODO(), filter)
	var cFilter common.Filter

	err = result.Decode(&cFilter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return &cFilter, nil
}

func RetrieveTweakIndexByHeight(blockHeight uint32) (*common.TweakIndex, error) {
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
	coll := client.Database("tweak_data").Collection("tweaks")
	filter := bson.D{{"block_height", blockHeight}}

	result := coll.FindOne(context.TODO(), filter)
	var index common.TweakIndex

	err = result.Decode(&index)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return &index, err
}

func DeleteLightUTXOByTxIndex(txId string, vout uint32) error {
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
	filter := bson.D{{"txid", txId}, {"vout", vout}}

	result, err := coll.DeleteOne(context.TODO(), filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Printf("Deleted %d LightUTXOs\n", result.DeletedCount)

	err = chainedTweakDeletion(client, txId)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	return nil
}

// chainedTweakDeletion chained deletion of tweak data if no more utxos with a certain txid are left
// runs whenever a light UTXO is deleted in order to keep the database lean and remove unneeded tweaks
func chainedTweakDeletion(client *mongo.Client, txId string) error {
	coll := client.Database("tweak_data").Collection("tweaks")
	filter := bson.D{{"txid", txId}}
	result := coll.FindOne(context.TODO(), filter)

	var utxo common.LightUTXO

	err := result.Decode(&utxo)
	if err != nil && err.Error() != "mongo: no documents in result" {
		common.ErrorLogger.Println(err)
		return err
	}

	// if no match was found
	if utxo.TxId == "" {
		var resultDelete *mongo.DeleteResult
		resultDelete, err = coll.DeleteOne(context.TODO(), filter)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		common.DebugLogger.Printf("Deleted %d core data for %s\n", resultDelete.DeletedCount, txId)
		return err
	}
	return nil
}

func CheckHeaderExists(blockHash string) (bool, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(common.MongoDBURI))
	if err != nil {
		common.ErrorLogger.Println(err)
		return false, err
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("headers").Collection("headers")
	var result common.BlockHeader

	// Use the hash to filter the documents
	filter := bson.D{{"hash", blockHash}}

	err = coll.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			common.ErrorLogger.Println(err)
			return false, err
		}
		common.ErrorLogger.Println(err)
		return false, err
	}

	// A document with the given hash exists
	return true, nil
}
