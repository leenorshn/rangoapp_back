package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Trans struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Amount     float64            `bson:"amount" json:"amount"`
	Operation  string             `bson:"operation" json:"operation"` // "Entree" or "Sortie"
	Libel      string             `bson:"libel" json:"libel"`
	OperatorID primitive.ObjectID `bson:"operatorId" json:"operatorId"`
	StoreID    primitive.ObjectID `bson:"storeId" json:"storeId"`
	Date       time.Time          `bson:"date" json:"date"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Caisse struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	In      float64            `bson:"in" json:"in"`
	Out     float64            `bson:"out" json:"out"`
	StoreID primitive.ObjectID `bson:"storeId" json:"storeId"`
}

// CreateTrans creates a new cash register transaction
func (db *DB) CreateTrans(operation string, amount float64, libel string, operatorID, storeID primitive.ObjectID) (*Trans, error) {
	transCollection := colHelper(db, "trans")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate operation
	if operation != "Entree" && operation != "Sortie" {
		return nil, gqlerror.Errorf("Operation must be 'Entree' or 'Sortie'")
	}

	// Validate amount
	if amount <= 0 {
		return nil, gqlerror.Errorf("Amount must be greater than 0")
	}

	trans := Trans{
		ID:         primitive.NewObjectID(),
		Amount:     amount,
		Operation:  operation,
		Libel:      libel,
		OperatorID: operatorID,
		StoreID:    storeID,
		Date:       time.Now(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err := transCollection.InsertOne(ctx, trans)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating transaction: %v", err)
	}

	return &trans, nil
}

// FindTransByStoreIDs finds all transactions for the given stores
func (db *DB) FindTransByStoreIDs(storeIDs []primitive.ObjectID) ([]*Trans, error) {
	transCollection := colHelper(db, "trans")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := transCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding transactions: %v", err)
	}
	defer cursor.Close(ctx)

	var trans []*Trans
	if err = cursor.All(ctx, &trans); err != nil {
		return nil, gqlerror.Errorf("Error decoding transactions: %v", err)
	}

	return trans, nil
}

// FindTransByID finds a transaction by ID
func (db *DB) FindTransByID(id string) (*Trans, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid transaction ID")
	}

	transCollection := colHelper(db, "trans")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var trans Trans
	err = transCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&trans)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Transaction not found")
		}
		return nil, gqlerror.Errorf("Error finding transaction: %v", err)
	}

	return &trans, nil
}

// FindCaisse calculates the cash register balance for a store
func (db *DB) FindCaisse(storeID string) (*Caisse, error) {
	storeObjectID, err := primitive.ObjectIDFromHex(storeID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	transactions, err := db.FindTransByStoreIDs([]primitive.ObjectID{storeObjectID})
	if err != nil {
		return nil, err
	}

	var mvntIn, mvntOut float64

	for _, t := range transactions {
		if t.Operation == "Entree" {
			mvntIn += t.Amount
		} else if t.Operation == "Sortie" {
			mvntOut += t.Amount
		}
	}

	return &Caisse{
		ID:      primitive.NewObjectID(),
		In:      mvntIn,
		Out:     mvntOut,
		StoreID: storeObjectID,
	}, nil
}

// FindCaisseByStoreIDs calculates cash register balances for multiple stores
func (db *DB) FindCaisseByStoreIDs(storeIDs []primitive.ObjectID) (*Caisse, error) {
	transactions, err := db.FindTransByStoreIDs(storeIDs)
	if err != nil {
		return nil, err
	}

	var mvntIn, mvntOut float64

	for _, t := range transactions {
		if t.Operation == "Entree" {
			mvntIn += t.Amount
		} else if t.Operation == "Sortie" {
			mvntOut += t.Amount
		}
	}

	return &Caisse{
		ID:      primitive.NewObjectID(),
		In:      mvntIn,
		Out:     mvntOut,
		StoreID: primitive.NilObjectID, // Multiple stores
	}, nil
}

// DeleteTrans deletes a transaction
func (db *DB) DeleteTrans(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid transaction ID")
	}

	transCollection := colHelper(db, "trans")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = transCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting transaction: %v", err)
	}

	return nil
}
