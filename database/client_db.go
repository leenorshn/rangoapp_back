package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Phone     string             `bson:"phone" json:"phone"`
	StoreID   primitive.ObjectID `bson:"storeId" json:"storeId"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateClient(name, phone string, storeID primitive.ObjectID) (*Client, error) {
	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := Client{
		ID:        primitive.NewObjectID(),
		Name:      name,
		Phone:     phone,
		StoreID:   storeID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := clientCollection.InsertOne(ctx, client)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating client: %v", err)
	}

	return &client, nil
}

func (db *DB) FindClientByID(id string) (*Client, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid client ID")
	}

	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var client Client
	err = clientCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&client)
	if err != nil {
		return nil, gqlerror.Errorf("Client not found")
	}

	return &client, nil
}

func (db *DB) FindClientsByStoreIDs(storeIDs []primitive.ObjectID) ([]*Client, error) {
	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := clientCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding clients: %v", err)
	}
	defer cursor.Close(ctx)

	var clients []*Client
	if err = cursor.All(ctx, &clients); err != nil {
		return nil, gqlerror.Errorf("Error decoding clients: %v", err)
	}

	return clients, nil
}

func (db *DB) UpdateClient(id string, name, phone *string) (*Client, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid client ID")
	}

	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if phone != nil {
		update["phone"] = *phone
	}

	_, err = clientCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating client: %v", err)
	}

	return db.FindClientByID(id)
}

func (db *DB) DeleteClient(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid client ID")
	}

	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = clientCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting client: %v", err)
	}

	return nil
}

