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
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) InsertProduct(newproduct model.NewProductInput, company string) (*model.Product, error) {
	productCollection := colHelper(db, "products")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return nil, gqlerror.Errorf("Erreur inconnue")
	}

	res, err := productCollection.InsertOne(ctx, bson.D{
		{Key: "priceOut", Value: newproduct.PriceOut},
		{Key: "imageUrl", Value: newproduct.ImageURL},
		{Key: "category", Value: newproduct.Category},
		{Key: "name", Value: newproduct.Name},
		{Key: "priceIn", Value: newproduct.PriceIn},
		{Key: "unite", Value: newproduct.Unite},
		{Key: "company", Value: companyId},
		{Key: "createdAt", Value: time.Now().Local().String()},
	})
	if err != nil {
		return nil, gqlerror.Errorf("Erreur d'insertion de donnee")
	}

	return &model.Product{
		ID:       res.InsertedID.(primitive.ObjectID).Hex(),
		Name:     newproduct.Name,
		ImageURL: newproduct.ImageURL,
		PriceIn:  newproduct.PriceIn,
		PriceOut: newproduct.PriceOut,
		Category: newproduct.Category,
		Unite:    newproduct.Unite,
	}, nil
}

func (db *DB) FindProducts(company string) []*model.Product {

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return nil
	}
	opts := options.Find().SetSort(bson.D{})
	cur, err := productCollection.Find(ctx, bson.D{{Key: "company", Value: companyId}}, opts)

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var products []*model.Product

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		var product *model.Product
		if err != nil {
			log.Fatal(err)
		}

		if err = cur.Decode(&product); err != nil {
			log.Fatal(err)
		}

		products = append(products, product)
	}

	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return products
}

func (db *DB) FindProduct(id string) *model.Product {

	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		panic(err)
	}
	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var product *model.Product

	err = productCollection.FindOne(ctx, bson.M{"_id": ObjectID}).Decode(&product)
	if err != nil {
		log.Fatal("product not found ", err)
	}

	return product

}

func (db *DB) UpdateProduct(id string, data *model.UpdateProductInput) (bool, error) {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filterCompany := bson.D{{Key: "_id", Value: ObjectID}}
	updateCompany := bson.D{{Key: "$set", Value: data}}
	resultCompany, err := productCollection.UpdateOne(ctx, filterCompany, updateCompany)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultCompany.ModifiedCount)
	return true, nil
}

func (db *DB) DeleteProduct(id string) (bool, error) {

	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	productCollection := colHelper(db, "products")
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
