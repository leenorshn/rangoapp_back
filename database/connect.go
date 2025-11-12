package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dbInstance *DB
)

type DB struct {
	client   *mongo.Client
	database *mongo.Database
}

func (db *DB) Client() *mongo.Client {
	return db.client
}

func ConnectDB() *DB {
	if dbInstance != nil {
		return dbInstance
	}

	dbUrl := os.Getenv("MONGO_URI")
	if dbUrl == "" {
		dbUrl = "mongodb+srv://leenor:avenir23@clusterzone1.b45aacv.mongodb.net/rangodb?retryWrites=true&w=majority"
	}

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "rangodb"
	}

	client, err := mongo.NewClient(options.Client().ApplyURI(dbUrl))
	if err != nil {
		log.Fatal("Error creating MongoDB client:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		log.Fatal("Error connecting to MongoDB:", err)
	}

	// Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Error pinging MongoDB:", err)
	}

	fmt.Println("Connected to MongoDB")

	dbInstance = &DB{
		client:   client,
		database: client.Database(dbName),
	}

	// Create indexes
	createIndexes(dbInstance)

	return dbInstance
}

func colHelper(db *DB, collectionName string) *mongo.Collection {
	return db.database.Collection(collectionName)
}

func createIndexes(db *DB) {
	ctx := context.Background()

	// Users indexes
	userCollection := colHelper(db, "users")
	userIndexes := []mongo.IndexModel{
		{
			Keys:    map[string]interface{}{"uid": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: map[string]interface{}{"companyId": 1},
		},
		{
			Keys: map[string]interface{}{"storeIds": 1},
		},
	}
	userCollection.Indexes().CreateMany(ctx, userIndexes)

	// Stores indexes
	storeCollection := colHelper(db, "stores")
	storeIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"companyId": 1},
		},
	}
	storeCollection.Indexes().CreateMany(ctx, storeIndexes)

	// Products indexes
	productCollection := colHelper(db, "products")
	productIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
	}
	productCollection.Indexes().CreateMany(ctx, productIndexes)

	// Clients indexes
	clientCollection := colHelper(db, "clients")
	clientIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
	}
	clientCollection.Indexes().CreateMany(ctx, clientIndexes)

	// Providers indexes
	providerCollection := colHelper(db, "providers")
	providerIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
	}
	providerCollection.Indexes().CreateMany(ctx, providerIndexes)

	// Factures indexes
	factureCollection := colHelper(db, "factures")
	factureIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{
				"storeId":       1,
				"factureNumber": 1,
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
	}
	factureCollection.Indexes().CreateMany(ctx, factureIndexes)

	// RapportStore indexes
	rapportCollection := colHelper(db, "rapportStore")
	rapportIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
		{
			Keys: map[string]interface{}{"productId": 1},
		},
	}
	rapportCollection.Indexes().CreateMany(ctx, rapportIndexes)

	fmt.Println("MongoDB indexes created")
}
