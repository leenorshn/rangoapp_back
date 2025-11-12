package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Provider struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Phone     string             `bson:"phone" json:"phone"`
	Address   string             `bson:"address" json:"address"`
	StoreID   primitive.ObjectID `bson:"storeId" json:"storeId"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateProvider(name, phone, address string, storeID primitive.ObjectID) (*Provider, error) {
	providerCollection := colHelper(db, "providers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider := Provider{
		ID:        primitive.NewObjectID(),
		Name:      name,
		Phone:     phone,
		Address:   address,
		StoreID:   storeID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := providerCollection.InsertOne(ctx, provider)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating provider: %v", err)
	}

	return &provider, nil
}

func (db *DB) FindProviderByID(id string) (*Provider, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid provider ID")
	}

	providerCollection := colHelper(db, "providers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var provider Provider
	err = providerCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&provider)
	if err != nil {
		return nil, gqlerror.Errorf("Provider not found")
	}

	return &provider, nil
}

func (db *DB) FindProvidersByStoreIDs(storeIDs []primitive.ObjectID) ([]*Provider, error) {
	providerCollection := colHelper(db, "providers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := providerCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding providers: %v", err)
	}
	defer cursor.Close(ctx)

	var providers []*Provider
	if err = cursor.All(ctx, &providers); err != nil {
		return nil, gqlerror.Errorf("Error decoding providers: %v", err)
	}

	return providers, nil
}

func (db *DB) UpdateProvider(id string, name, phone, address *string) (*Provider, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid provider ID")
	}

	providerCollection := colHelper(db, "providers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if phone != nil {
		update["phone"] = *phone
	}
	if address != nil {
		update["address"] = *address
	}

	_, err = providerCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating provider: %v", err)
	}

	return db.FindProviderByID(id)
}

func (db *DB) DeleteProvider(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid provider ID")
	}

	providerCollection := colHelper(db, "providers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = providerCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting provider: %v", err)
	}

	return nil
}

