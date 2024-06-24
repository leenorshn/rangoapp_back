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

func (db *DB) InsertCompany(newcompany model.NewCompanyInput) (*model.Company, error) {
	companyCollection := colHelper(db, "companies")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	company, err := db.CheckCompanyByName(newcompany.Name)
	if err != nil {
		return nil, gqlerror.Errorf("Erreur d'ecriture")
	}

	if company != nil {
		return nil, gqlerror.Errorf("This all ready existe")
	}

	res, err := companyCollection.InsertOne(ctx, bson.D{
		{Key: "isVerified", Value: true},
		{Key: "logo", Value: newcompany.Logo},
		{Key: "detail", Value: newcompany.Detail},
		{Key: "name", Value: newcompany.Name},
		{Key: "address", Value: newcompany.Address},
		{Key: "idNat", Value: newcompany.IDNat},
		{Key: "email", Value: newcompany.Email},
		{Key: "isBlocked", Value: false},
		{Key: "createdAt", Value: time.Now().Local().String()},
	})
	if err != nil {
		return nil, gqlerror.Errorf("Erreur d'insertion de donnee")
	}

	return &model.Company{
		ID:      res.InsertedID.(primitive.ObjectID).Hex(),
		Name:    newcompany.Name,
		Detail:  newcompany.Detail,
		Logo:    newcompany.Logo,
		Address: newcompany.Address,
		Email:   newcompany.Email,
		IDNat:   &newcompany.Type,
	}, nil
}

func (db *DB) FindCompanies() []*model.Company {

	companyCollection := colHelper(db, "companies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.Find().SetSort(bson.D{})
	cur, err := companyCollection.Find(ctx, bson.D{}, opts)

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var companies []*model.Company

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {

		var company *model.Company
		if err != nil {
			log.Fatal(err)
		}

		if err = cur.Decode(&company); err != nil {
			log.Fatal(err)
		}

		companies = append(companies, company)
	}

	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return companies
}

func (db *DB) FindCompany(name string) (*model.Company, error) {

	companyCollection := colHelper(db, "companies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var company *model.Company

	err := companyCollection.FindOne(ctx, bson.M{"name": name}).Decode(&company)
	if err != nil {
		return nil, gqlerror.Errorf("%s", err)
	}

	return company, nil

}

func (db *DB) UpdateCompanyData(id string, data *model.UpdateCompanyInput) (bool, error) {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	actionCollection := colHelper(db, "companies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filterCompany := bson.D{{Key: "_id", Value: ObjectID}}
	updateCompany := bson.D{{Key: "$set", Value: data}}
	resultCompany, err := actionCollection.UpdateOne(ctx, filterCompany, updateCompany)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultCompany.ModifiedCount)
	return true, nil
}

func (db *DB) CheckCompanyByName(name string) (*model.Company, error) {

	companyCollection := colHelper(db, "companies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var company *model.Company

	result := companyCollection.FindOne(ctx, bson.M{"name": name})
	err := result.Decode(&company)
	if err != nil {
		fmt.Println("Erreur company not found")

	}

	return company, nil

}
