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

func (db *DB) InsertTrans(operation string, amount float64, libel, company string, user model.User) (bool, error) {
	productCollection := colHelper(db, "trans")
	//caisseCollection := colHelper(db, "sales")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return false, gqlerror.Errorf("Erreur inconnue")
	}

	res, err := productCollection.InsertOne(ctx, bson.D{
		{Key: "amount", Value: amount},
		{Key: "libel", Value: libel},
		{Key: "operation", Value: operation},
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

func (db *DB) FindCaisseMouments(company string) []*model.Trans {

	tranCollection := colHelper(db, "trans")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return nil
	}
	opts := options.Find().SetSort(bson.D{{Key: "company", Value: companyId}})
	cur, err := tranCollection.Find(ctx, bson.D{}, opts)

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var trans []*model.Trans

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		var tran *model.Trans
		if err != nil {
			log.Fatal(err)
		}

		if err = cur.Decode(&tran); err != nil {
			log.Fatal(err)
		}

		trans = append(trans, tran)
	}

	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return trans
}

func (db *DB) FindCaisse(company string) *model.Caisse {

	sales := db.FindCaisseMouments(company)
	var mvntIn, mvntOut float64

	for _, s := range sales {
		if s.Operation == "Entree" {
			mvntIn = mvntIn + *s.Amount
		} else {
			mvntOut = mvntOut - *s.Amount
		}
	}

	return &model.Caisse{
		In:  mvntIn,
		Out: mvntOut,
	}

}
