package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MouvementStock struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID primitive.ObjectID `bson:"productId" json:"productId"`
	StoreID   primitive.ObjectID `bson:"storeId" json:"storeId"`
	Quantity  float64            `bson:"quantity" json:"quantity"`
	Operation string             `bson:"operation" json:"operation"` // "entree" or "sortie"
	Date      time.Time          `bson:"date" json:"date"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// CreateMouvementStock creates a new stock movement entry
func (db *DB) CreateMouvementStock(productID, storeID string, quantity float64, operation string) (*MouvementStock, error) {
	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	productObjectID, err := primitive.ObjectIDFromHex(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	storeObjectID, err := primitive.ObjectIDFromHex(storeID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	// Verify product belongs to store
	product, err := db.FindProductByID(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}
	if product.StoreID != storeObjectID {
		return nil, gqlerror.Errorf("Product does not belong to the specified store")
	}

	// Validate operation
	if operation != "entree" && operation != "sortie" {
		return nil, gqlerror.Errorf("Operation must be 'entree' or 'sortie'")
	}

	mouvement := MouvementStock{
		ID:        primitive.NewObjectID(),
		ProductID: productObjectID,
		StoreID:   storeObjectID,
		Quantity:  quantity,
		Operation: operation,
		Date:      time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = mouvementCollection.InsertOne(ctx, mouvement)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating mouvement stock: %v", err)
	}

	return &mouvement, nil
}

// FindMouvementStocksByStoreIDs finds all stock movements for products in the given stores
func (db *DB) FindMouvementStocksByStoreIDs(storeIDs []primitive.ObjectID) ([]*MouvementStock, error) {
	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := mouvementCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding mouvement stocks: %v", err)
	}
	defer cursor.Close(ctx)

	var mouvements []*MouvementStock
	if err = cursor.All(ctx, &mouvements); err != nil {
		return nil, gqlerror.Errorf("Error decoding mouvement stocks: %v", err)
	}

	return mouvements, nil
}

// FindMouvementStockByID finds a stock movement by ID
func (db *DB) FindMouvementStockByID(id string) (*MouvementStock, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid mouvement stock ID")
	}

	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mouvement MouvementStock
	err = mouvementCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&mouvement)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Mouvement stock not found")
		}
		return nil, gqlerror.Errorf("Error finding mouvement stock: %v", err)
	}

	return &mouvement, nil
}

// FindMouvementStocksByProductID finds all stock movements for a specific product
func (db *DB) FindMouvementStocksByProductID(productID primitive.ObjectID) ([]*MouvementStock, error) {
	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := mouvementCollection.Find(ctx, bson.M{"productId": productID})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding mouvement stocks: %v", err)
	}
	defer cursor.Close(ctx)

	var mouvements []*MouvementStock
	if err = cursor.All(ctx, &mouvements); err != nil {
		return nil, gqlerror.Errorf("Error decoding mouvement stocks: %v", err)
	}

	return mouvements, nil
}

// DeleteMouvementStock deletes a stock movement entry
func (db *DB) DeleteMouvementStock(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid mouvement stock ID")
	}

	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = mouvementCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting mouvement stock: %v", err)
	}

	return nil
}
