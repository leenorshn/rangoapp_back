package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RapportStore struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Type      string             `bson:"type" json:"type"` // "entree" ou "sortie"
	ProductID primitive.ObjectID `bson:"productId" json:"productId"`
	Quantity  float64            `bson:"quantity" json:"quantity"`
	Date      time.Time          `bson:"date" json:"date"`
	StoreID   primitive.ObjectID `bson:"storeId" json:"storeId"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateRapportStore(rapport *RapportStore) (*RapportStore, error) {
	rapportCollection := colHelper(db, "rapportStore")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rapport.CreatedAt = time.Now()
	rapport.UpdatedAt = time.Now()

	_, err := rapportCollection.InsertOne(ctx, rapport)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating rapport store: %v", err)
	}

	return rapport, nil
}

func (db *DB) FindRapportStoreByID(id string) (*RapportStore, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid rapport store ID")
	}

	rapportCollection := colHelper(db, "rapportStore")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rapport RapportStore
	err = rapportCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&rapport)
	if err != nil {
		return nil, gqlerror.Errorf("Rapport store not found")
	}

	return &rapport, nil
}

func (db *DB) FindRapportsByStoreIDs(storeIDs []primitive.ObjectID) ([]*RapportStore, error) {
	rapportCollection := colHelper(db, "rapportStore")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := rapportCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding rapports: %v", err)
	}
	defer cursor.Close(ctx)

	var rapports []*RapportStore
	if err = cursor.All(ctx, &rapports); err != nil {
		return nil, gqlerror.Errorf("Error decoding rapports: %v", err)
	}

	return rapports, nil
}

func (db *DB) DeleteRapportStore(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid rapport store ID")
	}

	rapportCollection := colHelper(db, "rapportStore")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = rapportCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting rapport store: %v", err)
	}

	return nil
}

