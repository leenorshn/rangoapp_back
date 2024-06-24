package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"rangoapp/utils"

	"rangoapp/graph/model"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) InsertUser(newUser model.NewUserInput, company string) (*model.User, error) {
	userCollection := colHelper(db, "users")

	companyId, err := primitive.ObjectIDFromHex(company)
	if err != nil {
		//fmt.Println("Error ", err)
		return nil, gqlerror.Errorf("Erreur inconnue")
	}
	password, err := utils.HashPassword(newUser.Password)
	if err != nil {
		//fmt.Println("Error ", err)
		return nil, gqlerror.Errorf("Password no conform")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user, err := db.FindUserByPhone(newUser.Phone)
	if err != nil {
		return nil, gqlerror.Errorf("Erreur d'ecriture")
	}
	if user != nil {
		return nil, gqlerror.Errorf("This account exist")
	}

	res, err := userCollection.InsertOne(ctx, bson.D{
		{Key: "avatar", Value: newUser.Avatar},
		{Key: "name", Value: newUser.Name},
		{Key: "phone", Value: newUser.Phone},
		{Key: "isBlocked", Value: false},
		{Key: "password", Value: password},
		{Key: "company", Value: companyId},
		{Key: "lastLogin", Value: time.Now().Local().String()},
		{Key: "createdAt", Value: time.Now().Local()},
	})
	if err != nil {
		return nil, gqlerror.Errorf("Erreur d'insertion de donnee")
	}

	return &model.User{
		ID:        res.InsertedID.(primitive.ObjectID).Hex(),
		Name:      newUser.Name,
		Phone:     newUser.Phone,
		Avatar:    newUser.Avatar,
		Address:   newUser.Address,
		IsBlocked: false,
		Password:  newUser.Password,
		LastLogin: time.Now().Local().String(),
	}, nil
}

func (db *DB) DeleteUser(id string) bool {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		panic(err)
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := userCollection.DeleteOne(ctx, bson.D{{Key: "_id", Value: ObjectID}})
	if err != nil {
		panic(err)
	}
	fmt.Printf("deleted %v documents\n", res.DeletedCount)
	return res.DeletedCount > 0

}

func (db *DB) FindUsers() []*model.User {

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// qry := []bson.M{

	// 	{
	// 		"$lookup": bson.M{

	// 			"from":         "companies",
	// 			"localField":   "company",
	// 			"foreignField": "_id",
	// 			"as":           "company",
	// 			// Arbitrary field name to store result set
	// 		},
	// 	},
	// 	{
	// 		"$unwind": "$company",
	// 	},
	// }

	//var todos []*model.Todo

	//cur, err := userCollection.Aggregate(ctx, qry)

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := userCollection.Find(ctx, bson.D{}, opts)

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var users []*model.User

	// Get a list of all returned documents and print them out.
	// See the mongo.Cursor documentation for more examples of using cursors.
	for cur.Next(ctx) {
		fmt.Println(cur.Current)

		var user *model.User
		if err != nil {
			log.Fatal(err)
		}

		if err = cur.Decode(&user); err != nil {
			log.Fatal(err)
		}

		users = append(users, user)
	}

	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return users
}

func (db *DB) FindUser(id string) *model.User {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Fatal(err)
	}
	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var user *model.User

	err = userCollection.FindOne(ctx, bson.D{{Key: "_id", Value: ObjectID}}).Decode(&user)

	if err != nil {
		log.Fatal("user not found ", err)
	}

	return user

}

func (db *DB) FindUserByPhone(phone string) (*model.User, error) {

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var user *model.User

	//opts := options.Find().SetSort(bson.D{})
	err := userCollection.FindOne(ctx, bson.D{{Key: "phone", Value: phone}}).Decode(&user)

	if err != nil {
		log.Fatal("user not found ", err)
	}

	return user, nil

}

func (db *DB) BlockUser(id string) (bool, error) {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	actionCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filterUser := bson.D{{Key: "_id", Value: ObjectID}}
	updateUser := bson.D{{Key: "$set", Value: bson.D{{Key: "isBlocked", Value: true}}}}
	resultUser, err := actionCollection.UpdateOne(ctx, filterUser, updateUser)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultUser.ModifiedCount)
	return true, nil
}

func (db *DB) UpdateUserData(id string, data *model.UpdateUserInput) (bool, error) {
	ObjectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	actionCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filterUser := bson.D{{Key: "_id", Value: ObjectID}}
	updateUser := bson.D{{Key: "$set", Value: data}}
	resultUser, err := actionCollection.UpdateOne(ctx, filterUser, updateUser)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultUser.ModifiedCount)
	return true, nil
}

func (db *DB) UpdatePassword(phone, code, password string) (bool, error) {

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user *model.User

	err := userCollection.FindOne(ctx, bson.M{"phone": phone, "code": code, "isVerified": false}).Decode(&user)
	if err != nil {
		//log.Fatal("code not found ", err)
		return false, gqlerror.Errorf("Code incorrect")
	}
	passwordNew, err := utils.HashPassword(password)
	if err != nil {
		fmt.Println("Error ", err)
		return false, gqlerror.Errorf("Password no conform")
	}

	filterUser := bson.D{{Key: "phone", Value: phone}}
	updateUser := bson.D{{Key: "$set", Value: bson.D{{Key: "password", Value: passwordNew}}}}
	resultUser, err := userCollection.UpdateOne(ctx, filterUser, updateUser)

	if err != nil {
		return false, err
	}
	fmt.Printf("%d", resultUser.ModifiedCount)
	return true, nil
}
