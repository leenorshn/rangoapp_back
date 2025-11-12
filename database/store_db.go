package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Store struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Address   string             `bson:"address" json:"address"`
	Phone     string             `bson:"phone" json:"phone"`
	CompanyID primitive.ObjectID `bson:"companyId" json:"companyId"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateStore(name, address, phone string, companyID primitive.ObjectID) (*Store, error) {
	storeCollection := colHelper(db, "stores")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := Store{
		ID:        primitive.NewObjectID(),
		Name:      name,
		Address:   address,
		Phone:     phone,
		CompanyID: companyID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := storeCollection.InsertOne(ctx, store)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating store: %v", err)
	}

	return &store, nil
}

func (db *DB) FindStoreByID(id string) (*Store, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var store Store
	err = storeCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&store)
	if err != nil {
		return nil, gqlerror.Errorf("Store not found")
	}

	return &store, nil
}

func (db *DB) FindStoresByCompanyID(companyID string) ([]*Store, error) {
	objectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := storeCollection.Find(ctx, bson.M{"companyId": objectID})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stores: %v", err)
	}
	defer cursor.Close(ctx)

	var stores []*Store
	if err = cursor.All(ctx, &stores); err != nil {
		return nil, gqlerror.Errorf("Error decoding stores: %v", err)
	}

	return stores, nil
}

func (db *DB) FindStoresByIDs(storeIDs []string) ([]*Store, error) {
	var objectIDs []primitive.ObjectID
	for _, id := range storeIDs {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		objectIDs = append(objectIDs, objectID)
	}

	if len(objectIDs) == 0 {
		return []*Store{}, nil
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := storeCollection.Find(ctx, bson.M{"_id": bson.M{"$in": objectIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stores: %v", err)
	}
	defer cursor.Close(ctx)

	var stores []*Store
	if err = cursor.All(ctx, &stores); err != nil {
		return nil, gqlerror.Errorf("Error decoding stores: %v", err)
	}

	return stores, nil
}

func (db *DB) UpdateStore(id string, name, address, phone *string) (*Store, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if address != nil {
		update["address"] = *address
	}
	if phone != nil {
		update["phone"] = *phone
	}

	_, err = storeCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating store: %v", err)
	}

	return db.FindStoreByID(id)
}

func (db *DB) DeleteStore(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid store ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if store has any products, clients, factures, etc.
	productCollection := colHelper(db, "products")
	productCount, _ := productCollection.CountDocuments(ctx, bson.M{"storeId": objectID})
	if productCount > 0 {
		return gqlerror.Errorf("Cannot delete store: it contains products")
	}

	clientCollection := colHelper(db, "clients")
	clientCount, _ := clientCollection.CountDocuments(ctx, bson.M{"storeId": objectID})
	if clientCount > 0 {
		return gqlerror.Errorf("Cannot delete store: it contains clients")
	}

	factureCollection := colHelper(db, "factures")
	factureCount, _ := factureCollection.CountDocuments(ctx, bson.M{"storeId": objectID})
	if factureCount > 0 {
		return gqlerror.Errorf("Cannot delete store: it contains factures")
	}

	_, err = storeCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting store: %v", err)
	}

	return nil
}

func (db *DB) VerifyStoreAccess(storeID, companyID string) (bool, error) {
	store, err := db.FindStoreByID(storeID)
	if err != nil {
		return false, err
	}

	companyObjectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return false, gqlerror.Errorf("Invalid company ID")
	}

	return store.CompanyID == companyObjectID, nil
}

