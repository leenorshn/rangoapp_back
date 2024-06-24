package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dbUrl = "mongodb://localhost:27017"
	//dbUrlCloud = "mongodb+srv://leenorshn:MGGGENpY7IU9ycJo@serverlessinstance0.njjjp.mongodb.net/?retryWrites=true&w=majority"
)

type DB struct {
	client *mongo.Client
}

func ConnectDB() *DB {
	client, err := mongo.NewClient(options.Client().ApplyURI(dbUrl))
	if err != nil {
		log.Fatal(err)
	}

	ctx, canceld := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	defer canceld()
	if err != nil {

		log.Fatal(err)
	}

	//ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB")
	return &DB{client: client}
}

func colHelper(db *DB, collectionName string) *mongo.Collection {
	return db.client.Database("rangoapp").Collection(collectionName)
}
