package mongodb

import (
	"SilentPaymentAppBackend/src/common"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const mongoDBURI = "mongodb://root:example@localhost:27017/"

// todo add unique lock
//  db.members.createIndex( { groupNumber: 1, lastname: 1, firstname: 1 }, { unique: true } )

func CreateIndices() {
	CreateIndexTransactions()
	CreateIndexCFilters()
	CreateIndexTweaks()
	CreateIndexUTXOs()
	CreateIndexSpentTXOs()
}

func CreateIndexTransactions() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
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
	fmt.Println("Created Index with name:", nameIndex)
}

func CreateIndexCFilters() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
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
	fmt.Println("Created Index with name:", nameIndex)
}

func CreateIndexUTXOs() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
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
	fmt.Println("Created Index with name:", nameIndex)
}

func CreateIndexSpentTXOs() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
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
	fmt.Println("Created Index with name:", nameIndex)
}

func CreateIndexTweaks() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
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
	fmt.Println("Created Index with name:", nameIndex)
}

func SaveTransactionDetails(transaction *common.Transaction) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("transactions").Collection("taproot_transactions")

	result, err := coll.InsertOne(context.TODO(), transaction)
	if err != nil {
		fmt.Println(err)
		//panic(err)
		return
	}

	fmt.Printf("Transaction inserted with ID: %s\n", result.InsertedID)
}

func SaveFilter(filter *common.Filter) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			fmt.Println(err.Error())
			return
		}
	}()

	coll := client.Database("filters").Collection("general")

	result, err := coll.InsertOne(context.TODO(), filter)
	if err != nil {
		//todo don't log duplicate keys as error but rather as debug
		fmt.Println(err.Error())
		return
	}

	fmt.Println("Filter inserted", "ID", result.InsertedID)
}

func SaveLightUTXO(utxo *common.LightUTXO) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("unspent")

	result, err := coll.InsertOne(context.TODO(), utxo)
	if err != nil {
		fmt.Println(err)
		//panic(err)
		return
	}

	fmt.Printf("UTXO inserted with ID: %s\n", result.InsertedID)
}

func SaveTweakData(tweak *common.TweakData) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("tweak_data").Collection("tweaks")

	result, err := coll.InsertOne(context.TODO(), tweak)
	if err != nil {
		fmt.Println(err)
		//panic(err)
		return
	}

	fmt.Printf("Tweak inserted with ID: %s\n", result.InsertedID)
}

func SaveSpentUTXO(utxo *common.SpentUTXO) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
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
		fmt.Println(err)
		return
	}

	fmt.Printf("Spent Transaction output inserted with ID: %s\n", result.InsertedID)
}

func RetrieveTransactionsByHeight(blockHeight uint32) []*common.Transaction {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("transactions").Collection("taproot_transactions")
	filter := bson.D{{"status.blockheight", blockHeight}}

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		fmt.Println(err)
	}

	var results []*common.Transaction
	if err = cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}

	return results
}

func RetrieveLightUTXOsByHeight(blockHeight uint32) []*common.LightUTXO {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	coll := client.Database("transaction_outputs").Collection("unspent")
	filter := bson.D{{"blockheight", blockHeight}}

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		fmt.Println(err)
	}

	var results []*common.LightUTXO
	if err = cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}

	return results
}

func RetrieveSpentUTXOsByHeight(blockHeight uint32) []*common.SpentUTXO {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	coll := client.Database("transaction_outputs").Collection("spent")
	filter := bson.D{{"blockheight", blockHeight}}

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		fmt.Println(err)
	}

	var results []*common.SpentUTXO
	if err = cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}

	return results
}

func RetrieveCFilterByHeight(blockHeight uint32) *common.Filter {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	coll := client.Database("filters").Collection("general")
	filter := bson.D{{"blockheight", blockHeight}}

	result := coll.FindOne(context.TODO(), filter)
	var cFilter common.Filter

	err = result.Decode(&cFilter)
	if err != nil {
		fmt.Println(err)
	}

	return &cFilter
}

func RetrieveTweakDataByHeight(blockHeight uint32) []*common.TweakData {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	coll := client.Database("tweak_data").Collection("tweaks")
	filter := bson.D{{"blockheight", blockHeight}}

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		fmt.Println(err)
	}

	var results []*common.TweakData
	if err = cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}

	return results
}

func DeleteLightUTXOByTxIndex(txId string, vout uint32) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDBURI))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("transaction_outputs").Collection("unspent")
	filter := bson.D{{"txid", txId}, {"vout", vout}}

	result, err := coll.DeleteOne(context.TODO(), filter)
	if err != nil {
		fmt.Println(err)
		//panic(err)
		return
	}

	fmt.Printf("Deleted %d LightUTXOs\n", result.DeletedCount)
}
