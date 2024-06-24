package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"rangoapp/graph/model"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (db *DB) InsertStock(product string, quantity, stockMin float64, company string) (bool, error) {
	stockCollection := colHelper(db, "stock")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return false, gqlerror.Errorf("Erreur inconnue")
	}

	productId, err := primitive.ObjectIDFromHex(product)
	if err != nil {
		//fmt.Println("Error ", err)
		return false, gqlerror.Errorf("Erreur inconnue")
	}

	res, err := stockCollection.InsertOne(ctx, bson.D{
		{Key: "product", Value: productId},
		{Key: "stockMin", Value: stockMin},
		{Key: "quantity", Value: quantity},
		{Key: "company", Value: companyId},
		{Key: "date", Value: time.Now().Local().String()},
	})

	if err != nil {
		return false, gqlerror.Errorf("Erreur d'insertion de donnee")
	}

	fmt.Println(res.InsertedID)

	return true,
		nil
}

func (db *DB) FindStocks(company string) []*model.Stock {

	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return nil
	}

	matchStage := bson.M{"$match": bson.D{{Key: "company", Value: companyId}}}

	qry := []bson.M{
		matchStage,
		{
			"$lookup": bson.M{

				"from":         "products",
				"localField":   "product",
				"foreignField": "_id",
				"as":           "product",
				// Arbitrary field name to store result set
			},
		},
		{
			"$unwind": "$product",
		},
	}

	//var todos []*model.Todo

	cur, err := stockCollection.Aggregate(ctx, qry)
	// opts := options.Find().SetSort(bson.D{{Key: "company", Value: companyId}})
	// cur, err := stockCollection.Find(ctx, bson.D{}, opts)

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var stocks []*model.Stock

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		var stock *model.Stock
		if err != nil {
			log.Fatal(err)
		}

		if err = cur.Decode(&stock); err != nil {
			log.Fatal(err)
		}

		stocks = append(stocks, stock)
	}

	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return stocks
}

func (db *DB) FindStockByProduct(id primitive.ObjectID) (*model.Stock, error) {

	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	matchStage := bson.M{"$match": bson.D{{Key: "product", Value: id}}}

	qry := []bson.M{
		matchStage,
		{
			"$lookup": bson.M{

				"from":         "products",
				"localField":   "product",
				"foreignField": "_id",
				"as":           "product",
				// Arbitrary field name to store result set
			},
		},
		{
			"$unwind": "$product",
		},
	}

	// err = stockCollection.FindOne(ctx, bson.M{"_id": ObjectID}).Decode(&stock)
	cur, err := stockCollection.Aggregate(ctx, qry)
	// opts := options.Find().SetSort(bson.D{})
	// cur, err := mouvementCollection.Find(ctx, bson.D{{Key: "company", Value: companyId}}, opts)

	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var stock *model.Stock

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		if err = cur.Decode(&stock); err != nil {
			return nil, err
		}
	}
	return stock, nil

}

func (db *DB) FindStock(id string) (*model.Stock, error) {

	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		panic(err)
	}
	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	matchStage := bson.M{"$match": bson.D{{Key: "_id", Value: ObjectID}}}

	qry := []bson.M{
		matchStage,
		{
			"$lookup": bson.M{

				"from":         "products",
				"localField":   "product",
				"foreignField": "_id",
				"as":           "product",
				// Arbitrary field name to store result set
			},
		},
		{
			"$unwind": "$product",
		},
	}

	// err = stockCollection.FindOne(ctx, bson.M{"_id": ObjectID}).Decode(&stock)
	cur, err := stockCollection.Aggregate(ctx, qry)
	// opts := options.Find().SetSort(bson.D{})
	// cur, err := mouvementCollection.Find(ctx, bson.D{{Key: "company", Value: companyId}}, opts)

	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var stock *model.Stock

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		if err = cur.Decode(&stock); err != nil {
			return nil, err
		}
	}
	return stock, nil

}

func (db *DB) UpdateStock(id string, quantity float64) (bool, error) {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	productCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	currentStock, erro := db.FindStock(id)
	if erro != nil {
		return false, erro
	}

	filterStock := bson.D{{Key: "_id", Value: ObjectID}}
	updateStock := bson.D{{Key: "$set", Value: bson.D{{Key: "quantity", Value: currentStock.Quantity + quantity}}}}
	resultStock, err := productCollection.UpdateOne(ctx, filterStock, updateStock)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultStock.ModifiedCount)
	return true, nil
}
func (db *DB) UpdateStockInAction(id string, quantity float64) (bool, error) {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	currentStock, erro := db.FindStockByProduct(ObjectID)
	if erro != nil {
		return false, erro
	}

	filterStock := bson.D{{Key: "product", Value: ObjectID}}
	updateStock := bson.D{{Key: "$set", Value: bson.D{{Key: "quantity", Value: currentStock.Quantity + quantity}}}}
	resultStock, err := stockCollection.UpdateOne(ctx, filterStock, updateStock)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultStock.ModifiedCount)
	return true, nil
}

func (db *DB) DeleteProductInStock(id string) (bool, error) {

	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	productCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filterProduct := bson.D{{Key: "_id", Value: ObjectID}}
	//updateProduct := bson.D{{Key: "$set", Value: bson.D{{Key: "isDeleted", Value: true}}}}
	resultProduct, err := productCollection.DeleteOne(ctx, filterProduct)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultProduct.DeletedCount)
	return true, nil

}
