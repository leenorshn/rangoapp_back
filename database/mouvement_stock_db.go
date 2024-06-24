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

func (db *DB) CreateMouvementStock(idProduct string, quantity float64, operation string, company string) (bool, error) {
	mouvementCollection := colHelper(db, "mouvements_stock")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return false, gqlerror.Errorf("Erreur inconnue")
	}

	productId, err := primitive.ObjectIDFromHex(idProduct)
	if err != nil {
		//fmt.Println("Error ", err)
		return false, gqlerror.Errorf("Erreur inconnue")
	}

	res, err := mouvementCollection.InsertOne(ctx, bson.D{
		{Key: "product", Value: productId},
		{Key: "quantity", Value: quantity},
		{Key: "operation", Value: operation},
		{Key: "company", Value: companyId},
		{Key: "date", Value: time.Now().Local().String()},
	})
	if err != nil {
		return false, gqlerror.Errorf("Erreur d'insertion de donnee")
	}

	fmt.Println(res.InsertedID)

	return true, nil
}

func (db *DB) GetMouvementStocks(company string) []*model.MouvementStock {

	mouvementCollection := colHelper(db, "mouvements_stock")
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

	cur, err := mouvementCollection.Aggregate(ctx, qry)
	// opts := options.Find().SetSort(bson.D{})
	// cur, err := mouvementCollection.Find(ctx, bson.D{{Key: "company", Value: companyId}}, opts)

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var mouvements []*model.MouvementStock

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		var mouvement *model.MouvementStock
		if err != nil {
			log.Fatal(err)
		}

		if err = cur.Decode(&mouvement); err != nil {
			log.Fatal(err)
		}

		mouvements = append(mouvements, mouvement)
	}

	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return mouvements
}
