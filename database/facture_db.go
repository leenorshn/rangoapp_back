package database

import (
	"context"
	"fmt"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FactureProduct struct {
	ProductID primitive.ObjectID `bson:"productId" json:"productId"`
	Quantity  int                 `bson:"quantity" json:"quantity"`
	Price     float64             `bson:"price" json:"price"`
}

type Facture struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FactureNumber string             `bson:"factureNumber" json:"factureNumber"`
	Products      []FactureProduct   `bson:"products" json:"products"`
	Quantity      int                `bson:"quantity" json:"quantity"`
	Date          time.Time          `bson:"date" json:"date"`
	Price         float64            `bson:"price" json:"price"`
	Currency      string             `bson:"currency" json:"currency"`
	ClientID      primitive.ObjectID `bson:"clientId" json:"clientId"`
	StoreID       primitive.ObjectID `bson:"storeId" json:"storeId"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) GenerateFactureNumber(storeID primitive.ObjectID) (string, error) {
	factureCollection := colHelper(db, "factures")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Count existing factures for this store
	count, err := factureCollection.CountDocuments(ctx, bson.M{"storeId": storeID})
	if err != nil {
		return "", gqlerror.Errorf("Error counting factures: %v", err)
	}

	// Generate unique number: FACT-{STORE_ID}-{YYYY}-{NUMERO}
	year := time.Now().Year()
	factureNumber := fmt.Sprintf("FACT-%s-%d-%d", storeID.Hex()[:8], year, count+1)

	return factureNumber, nil
}

func (db *DB) CreateFacture(facture *Facture) (*Facture, error) {
	factureCollection := colHelper(db, "factures")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate facture number
	factureNumber, err := db.GenerateFactureNumber(facture.StoreID)
	if err != nil {
		return nil, err
	}
	facture.FactureNumber = factureNumber
	facture.CreatedAt = time.Now()
	facture.UpdatedAt = time.Now()

	_, err = factureCollection.InsertOne(ctx, facture)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Retry with new number if duplicate
			factureNumber, err = db.GenerateFactureNumber(facture.StoreID)
			if err != nil {
				return nil, err
			}
			facture.FactureNumber = factureNumber
			_, err = factureCollection.InsertOne(ctx, facture)
			if err != nil {
				return nil, gqlerror.Errorf("Error creating facture: %v", err)
			}
		} else {
			return nil, gqlerror.Errorf("Error creating facture: %v", err)
		}
	}

	return facture, nil
}

func (db *DB) FindFactureByID(id string) (*Facture, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid facture ID")
	}

	factureCollection := colHelper(db, "factures")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var facture Facture
	err = factureCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&facture)
	if err != nil {
		return nil, gqlerror.Errorf("Facture not found")
	}

	return &facture, nil
}

func (db *DB) FindFacturesByStoreIDs(storeIDs []primitive.ObjectID) ([]*Facture, error) {
	factureCollection := colHelper(db, "factures")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := factureCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding factures: %v", err)
	}
	defer cursor.Close(ctx)

	var factures []*Facture
	if err = cursor.All(ctx, &factures); err != nil {
		return nil, gqlerror.Errorf("Error decoding factures: %v", err)
	}

	return factures, nil
}

func (db *DB) UpdateFacture(id string, products []FactureProduct, clientID *primitive.ObjectID, quantity *int, price *float64, currency *string, date *time.Time) (*Facture, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid facture ID")
	}

	factureCollection := colHelper(db, "factures")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"updatedAt": time.Now()}
	if products != nil {
		update["products"] = products
	}
	if clientID != nil {
		update["clientId"] = *clientID
	}
	if quantity != nil {
		update["quantity"] = *quantity
	}
	if price != nil {
		update["price"] = *price
	}
	if currency != nil {
		update["currency"] = *currency
	}
	if date != nil {
		update["date"] = *date
	}

	_, err = factureCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating facture: %v", err)
	}

	return db.FindFactureByID(id)
}

func (db *DB) DeleteFacture(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid facture ID")
	}

	factureCollection := colHelper(db, "factures")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = factureCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting facture: %v", err)
	}

	return nil
}

