package database

import (
	"context"
	"fmt"
	"log"
	"rangoapp/graph/model"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) InsertSale(newproduct model.NewSaleInput, company string, user model.User) (bool, error) {
	productCollection := colHelper(db, "sales")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return false, gqlerror.Errorf("Erreur inconnue")
	}

	res, err := productCollection.InsertOne(ctx, bson.D{
		{Key: "basket", Value: newproduct.Basket},
		{Key: "priceToPay", Value: newproduct.PriceToPay},
		{Key: "pricePayed", Value: newproduct.PricePayed},
		{Key: "client", Value: newproduct.Client},
		{Key: "operator", Value: user},
		{Key: "company", Value: companyId},
		{Key: "date", Value: time.Now().Local().String()},
	})
	if err != nil {
		return false, gqlerror.Errorf("Erreur d'insertion de donnee")
	}
	fmt.Println(res.InsertedID)

	return true, nil
}

func (db *DB) FindSales(company string) []*model.Sale {

	saleCollection := colHelper(db, "sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return nil
	}
	opts := options.Find().SetSort(bson.D{{Key: "company", Value: companyId}})
	cur, err := saleCollection.Find(ctx, bson.D{}, opts)

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var sales []*model.Sale

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		var sale *model.Sale
		if err != nil {
			log.Fatal(err)
		}

		if err = cur.Decode(&sale); err != nil {
			log.Fatal(err)
		}

		sales = append(sales, sale)
	}

	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return sales
}

func (db *DB) FindSale(id string) *model.Sale {

	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		panic(err)
	}
	saleCollection := colHelper(db, "sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var sale *model.Sale

	err = saleCollection.FindOne(ctx, bson.M{"_id": ObjectID}).Decode(&sale)
	if err != nil {
		log.Fatal("sale not found ", err)
	}

	return sale

}
